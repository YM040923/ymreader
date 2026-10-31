#!/usr/bin/env python3
"""Build and inspect a Docker-based fnOS package using the official fnpack."""
import argparse
from datetime import datetime, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent

BRANDING_DEFAULTS = {
    "appname": "nowen-reader",
    "display_name": "弄文阅读",
    "service_port": "6680",
    "icon_prefix": "nowen-reader-fnos-icon",
    "maintainer": "Nowen",
    "maintainer_url": "https://github.com/cropflre/nowen-reader",
    "distributor": "Nowen",
    "distributor_url": "https://github.com/cropflre/nowen-reader",
}

TEMPLATE_FILES = (
    "manifest",
    "app/ui/config",
    "config/resource",
    "config/privilege",
    "app/docker/docker-compose.yaml",
    *[f"cmd/{name}" for name in (
        "main", "install_init", "install_callback", "upgrade_init", "upgrade_callback",
        "uninstall_init", "uninstall_callback", "config_init", "config_callback",
    )],
)


def png_size(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError("Expected a PNG icon")
    return struct.unpack(">II", data[16:24])


def archive_files(archive):
    result = {}
    for member in archive.getmembers():
        if member.isfile():
            name = member.name.removeprefix("./")
            result[name] = (member, archive.extractfile(member).read())
    return result


def resolve_branding(release, appname=None, display_name=None, port=None, icon_prefix=None):
    branding = {key: release.get(key, value) for key, value in BRANDING_DEFAULTS.items()}
    if appname:
        branding["appname"] = appname
    if display_name:
        branding["display_name"] = display_name
    if port:
        branding["service_port"] = str(port)
    if icon_prefix:
        branding["icon_prefix"] = icon_prefix
    branding["desktop_app"] = f"{branding['appname']}.Application"
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,31}", branding["appname"]):
        raise ValueError("App name must be lowercase letters, digits, or hyphens")
    if not branding["display_name"].strip() or "`" in branding["display_name"]:
        raise ValueError("Display name must be non-empty and cannot contain a backtick")
    if not re.fullmatch(r"[1-9][0-9]{0,4}", branding["service_port"]) or int(branding["service_port"]) > 65535:
        raise ValueError("Service port must be 1-65535")
    return branding


def render_templates(source, version, image, description, branding, platform="all"):
    replacements = {
        "VERSION": version,
        "DESCRIPTION": description,
        "IMAGE": image,
        "PLATFORM": platform,
        **{key.upper(): value for key, value in branding.items()},
    }
    for name in TEMPLATE_FILES:
        path = source / name
        text = path.read_text(encoding="utf-8")
        for key, value in replacements.items():
            text = text.replace("{{" + key + "}}", value)
        if re.search(r"\{\{[A-Z_]+\}\}", text):
            raise ValueError(f"Unresolved template variable: {name}")
        path.write_text(text, encoding="utf-8", newline="\n")


def normalize_modes(source):
    # fnOS reads package files as its own service user; files produced by a
    # 0700 umask (manifest, config, icons) must not stay private to the builder.
    for path in source.rglob("*"):
        path.chmod(0o755 if path.is_dir() else 0o644)
    for path in (source / "cmd").iterdir():
        path.chmod(0o755)


def verify_package(package, version, image, description, branding, platform="all"):
    appname = branding["appname"]
    with tarfile.open(package, "r:*") as outer:
        files = archive_files(outer)
    manifest = files["manifest"][1].decode("utf-8")
    fields = {}
    for line in manifest.splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            fields[key.strip()] = value.strip().strip("`")
    expected = {"appname": appname, "display_name": branding["display_name"], "version": version,
                "platform": platform, "desktop_applaunchname": branding["desktop_app"],
                "service_port": branding["service_port"]}
    for key, value in expected.items():
        if fields.get(key) != value:
            raise ValueError(f"Manifest mismatch: {key}")
    if fields.get("desc") != description:
        raise ValueError("Packaged application description was changed or truncated")
    for name, size in (("ICON.PNG", 64), ("ICON_256.PNG", 256)):
        if png_size(files[name][1]) != (size, size):
            raise ValueError(f"Incorrect icon size: {name}")
    # fnpack stores app/ as a nested archive; cmd/ and config/ are outer files.
    with tarfile.open(fileobj=io.BytesIO(files["app.tgz"][1]), mode="r:*") as app:
        app_files = archive_files(app)
    compose = app_files["docker/docker-compose.yaml"][1].decode("utf-8")
    if f"image: {image}" not in compose or "{{IMAGE}}" in compose:
        raise ValueError("Packaged Docker image does not match release.json")
    if f"container_name: {appname}" not in compose:
        raise ValueError("Compose container name does not match the app name")
    for token in ("TRIM_PKGVAR", "TRIM_DATA_SHARE_PATHS", "TRIM_UID", "TRIM_GID"):
        if token not in compose:
            raise ValueError(f"Missing fnOS runtime variable: {token}")
    ui = json.loads(app_files["ui/config"][1])[".url"][branding["desktop_app"]]
    if ui["port"] != branding["service_port"] or ui["protocol"] != "http" or ui["title"] != branding["display_name"]:
        raise ValueError("Desktop entry name, protocol, or service port disagrees")
    for size in (64, 256):
        if app_files[f"ui/images/icon_{size}.png"][1] != files["ICON.PNG" if size == 64 else "ICON_256.PNG"][1]:
            raise ValueError("Desktop and application center icons disagree")
    privilege = json.loads(files["config/privilege"][1])
    if privilege.get("username") != f"docker-{appname}" or privilege.get("groupname") != f"docker-{appname}":
        raise ValueError("Package privilege identity does not match the app name")
    resource = json.loads(files["config/resource"][1])
    if resource["data-share"]["shares"] != [{"name": appname}]:
        raise ValueError("Compose requires exactly one application book share")
    for hook in ("main", "install_init", "install_callback", "upgrade_init", "upgrade_callback",
                 "uninstall_init", "uninstall_callback", "config_init", "config_callback"):
        if f"cmd/{hook}" not in files:
            raise ValueError(f"Missing lifecycle hook: {hook}")
    if b"TRIM_PKGVAR" in files["cmd/install_init"][1] or b"mkdir" in files["cmd/install_init"][1]:
        raise ValueError("install_init must not create runtime directories before fnOS prepares them")
    for hook in ("install_callback", "upgrade_init"):
        if b'${TRIM_PKGVAR}/cache/pages' not in files[f"cmd/{hook}"][1]:
            raise ValueError("Nested page cache must be prepared by the package user")
    for name, (member, content) in files.items():
        if name.startswith("cmd/"):
            if not member.mode & 0o111 or not content.startswith(b"#!/bin/bash\n") or b"\r" in content:
                raise ValueError(f"Invalid lifecycle script permissions or line endings: {name}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fnpack", default=os.environ.get("FNPACK_BIN") or shutil.which("fnpack"))
    parser.add_argument("--out", type=Path, default=ROOT / "dist/fpk")
    parser.add_argument("--version", help="Package version; defaults to release.json")
    parser.add_argument("--image", help="Versioned image reference with registry digest; defaults to release.json")
    parser.add_argument("--appname", help="fnOS app name, e.g. ymreader")
    parser.add_argument("--display-name", help="Application center display name")
    parser.add_argument("--port", type=int, help="Exposed service port; defaults to 6680")
    parser.add_argument("--icon-prefix", help="Icon basename under docs/assets, without -fnos-icon-N.png")
    parser.add_argument("--platform", choices=["x86", "arm", "all"], default="all",
                        help="fnOS platform label; declare only architectures the image actually supports")
    args = parser.parse_args()
    if not args.fnpack:
        parser.error("Pass --fnpack /path/to/official/fnpack or set FNPACK_BIN")
    fnpack = str(Path(args.fnpack).resolve())
    release = json.loads((HERE / "release.json").read_text(encoding="utf-8"))
    version = args.version or release["version"]
    image = args.image or release["image"]
    branding = resolve_branding(release, args.appname, args.display_name, args.port, args.icon_prefix)
    if not re.fullmatch(r"\d+\.\d+\.\d+", version):
        raise ValueError("FPK version must be X.Y.Z")
    if not re.fullmatch(rf"[A-Za-z0-9][A-Za-z0-9._/@:-]*:v{re.escape(version)}@sha256:[a-f0-9]{{64}}", image):
        raise ValueError("The versioned image must include its immutable registry digest")
    description = "".join(line.strip() for line in (HERE / "app-description.html").read_text(encoding="utf-8").splitlines())
    if "`" in description:
        raise ValueError("Description cannot contain fnpack's backtick delimiter")
    args.out = args.out.resolve()
    args.out.mkdir(parents=True, exist_ok=True)
    appname = branding["appname"]
    with tempfile.TemporaryDirectory(prefix=f"{appname}-fpk-") as temporary:
        temporary = Path(temporary)
        source = temporary / appname
        shutil.copytree(HERE / "template", source)
        render_templates(source, version, image, description, branding, args.platform)
        for path in (source / "cmd").iterdir():
            path.write_text(path.read_text(encoding="utf-8"), encoding="utf-8", newline="\n")
            path.chmod(0o755)
        icons = source / "app/ui/images"
        icons.mkdir(parents=True, exist_ok=True)
        for size, filename in ((64, "ICON.PNG"), (256, "ICON_256.PNG")):
            icon = ROOT / f"docs/assets/{branding['icon_prefix']}-{size}.png"
            if png_size(icon.read_bytes()) != (size, size):
                raise ValueError(f"Incorrect source icon size: {icon}")
            shutil.copyfile(icon, source / filename)
            shutil.copyfile(icon, icons / f"icon_{size}.png")
        normalize_modes(source)
        tool = subprocess.run([fnpack, "--help"], check=True, capture_output=True, text=True).stdout
        print(tool.split("Usage:")[0].strip(), flush=True)
        subprocess.run([fnpack, "build", "-d", str(source)], cwd=temporary, check=True)
        candidates = list(temporary.rglob("*.fpk"))
        if len(candidates) != 1:
            raise ValueError(f"Expected one generated FPK, found {len(candidates)}")
        package = candidates[0]
        verify_package(package, version, image, description, branding, args.platform)
        destination = args.out / f"{appname}-{version}.fpk"
        shutil.copyfile(package, destination)
    digest = hashlib.sha256(destination.read_bytes()).hexdigest()
    (args.out / "SHA256SUMS").write_text(f"{digest}  {destination.name}\n", encoding="utf-8", newline="\n")
    metadata = {**release, **branding, "version": version, "image": image, "platform": args.platform,
                "package": destination.name,
                "bytes": destination.stat().st_size,
                "sha256": digest, "built_at": datetime.now(timezone.utc).isoformat(),
                "fnpack": tool.split("Usage:")[0].strip(), "package_contents_verified": True,
                "physical_fnos_acceptance": "not performed"}
    (args.out / "package-info.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"Verified: {destination}\nSHA-256: {digest}")


if __name__ == "__main__":
    main()
