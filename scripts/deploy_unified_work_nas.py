#!/usr/bin/env python3
"""Safely build and deploy the isolated YMReader unified-work test container.

Dry-run is the default. Network or Docker changes require both ``--execute``
and the exact confirmation token printed by ``--help``.
"""

from __future__ import annotations

import argparse
import dataclasses
import getpass
import http.cookiejar
import io
import json
import os
import shlex
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path, PurePosixPath

from bootstrap_unified_work_fixture import build_fixture


PRODUCTION_PORT = 6680
CONFIRMATION_TOKEN = "ymreader-unified-work-clean@192.168.2.9:17680"
ALLOWED_REMOTE_ROOT = PurePosixPath("/vol3/1000/ymreader-unified-work-clean")


@dataclasses.dataclass(frozen=True)
class DeploymentPlan:
    host: str = "192.168.2.9"
    ssh_port: int = 22
    ssh_user: str = "ymzwh"
    port: int = 17680
    image: str = "ymreader:unified-work-clean"
    container: str = "ymreader-unified-work-clean"
    remote_root: str = str(ALLOWED_REMOTE_ROOT)
    puid: int = 1000
    pgid: int = 1000
    timezone: str = "Asia/Shanghai"
    admin_username: str = "admin"
    admin_password: str = dataclasses.field(default="", repr=False)
    library_name: str = "Unified Work Parallel Fixture"

    @property
    def base_url(self) -> str:
        return f"http://{self.host}:{self.port}"

    @property
    def ssh_destination(self) -> str:
        return f"{self.ssh_user}@{self.host}"


@dataclasses.dataclass(frozen=True)
class RemoteCommandPlan:
    commands: tuple[str, ...]
    destructive: bool = False


def validate_plan(plan: DeploymentPlan) -> None:
    if plan.host != "192.168.2.9":
        raise ValueError("this deployment script is locked to NAS 192.168.2.9")
    if plan.port == PRODUCTION_PORT:
        raise ValueError("production port 6680 is permanently forbidden")
    if plan.port != 17680:
        raise ValueError("the unified-work test container must use port 17680")
    if plan.image != "ymreader:unified-work-clean":
        raise ValueError("unexpected image tag")
    if plan.container != "ymreader-unified-work-clean":
        raise ValueError("unexpected container name")
    if PurePosixPath(plan.remote_root) != ALLOWED_REMOTE_ROOT:
        raise ValueError("all test data must stay under the fixed vol3 root")
    if "/vol1/" in plan.remote_root or plan.remote_root.startswith("/vol1"):
        raise ValueError("vol1 is forbidden")
    if plan.ssh_port <= 0 or plan.ssh_port > 65535:
        raise ValueError("invalid SSH port")
    if min(plan.puid, plan.pgid) < 0:
        raise ValueError("PUID and PGID must be non-negative")


def resolve_admin_password(interactive: bool) -> str:
    password = os.environ.get("YMREADER_ADMIN_PASSWORD", "")
    if password:
        return password
    if not interactive:
        return ""
    if not sys.stdin.isatty():
        raise RuntimeError(
            "YMREADER_ADMIN_PASSWORD is required when no interactive terminal is available"
        )
    password = getpass.getpass("YMReader test administrator password: ")
    if not password:
        raise RuntimeError("administrator password cannot be empty")
    return password


def render_compose(plan: DeploymentPlan) -> str:
    validate_plan(plan)
    root = plan.remote_root
    compose = f"""services:
  ymreader-unified-work-clean:
    image: {plan.image}
    container_name: {plan.container}
    restart: unless-stopped
    ports:
      - "{plan.port}:3000"
    environment:
      GIN_MODE: release
      PORT: "3000"
      DATABASE_URL: /data/nowen-reader.db
      COMICS_DIR: /app/comics/_default-empty
      NOVELS_DIR: /app/novels
      DATA_DIR: /app/.cache
      PUID: "{plan.puid}"
      PGID: "{plan.pgid}"
      TZ: {plan.timezone}
    volumes:
      - {root}/data:/data
      - {root}/cache:/app/.cache
      - {root}/comics:/app/comics
      - {root}/novels:/app/novels
    logging:
      driver: json-file
      options:
        max-size: 10m
        max-file: "3"
"""
    forbidden = ("6680:", "/vol1/", "volumes:\n      - /data")
    if any(value in compose for value in forbidden):
        raise AssertionError("unsafe compose output")
    for target in (":/data", ":/app/.cache", ":/app/comics", ":/app/novels"):
        if target not in compose:
            raise AssertionError(f"missing explicit bind mount {target}")
    return compose


def shell_quote(value: str) -> str:
    return shlex.quote(value)


def build_remote_commands(
    plan: DeploymentPlan,
    execute: bool = False,
) -> RemoteCommandPlan:
    validate_plan(plan)
    root = shell_quote(plan.remote_root)
    image = shell_quote(plan.image)
    container = shell_quote(plan.container)
    commands = (
        f"test {root} = /vol3/1000/ymreader-unified-work-clean",
        f"mkdir -p {root}/staging {root}/build",
        f"rm -rf {root}/build/source.next",
        f"mkdir -p {root}/build/source.next",
        f"tar -xzf {root}/staging/source.tar.gz -C {root}/build/source.next",
        f"rm -rf {root}/build/source.previous",
        f"if [ -d {root}/build/source ]; then mv {root}/build/source "
        f"{root}/build/source.previous; fi",
        f"mv {root}/build/source.next {root}/build/source",
        f"docker build --pull --tag {image} "
        f"--build-arg VERSION=unified-work-clean {root}/build/source",
        f"docker rm -f {container} >/dev/null 2>&1 || true",
        # The instance is an acceptance fixture, so every deployment starts
        # from an empty isolated database/cache and cannot inherit stale scans.
        f"rm -rf {root}/data {root}/cache {root}/comics/fixture {root}/novels",
        f"mkdir -p {root}/data {root}/cache {root}/comics/fixture "
        f"{root}/comics/_default-empty {root}/novels",
        f"tar -xzf {root}/staging/fixture.tar.gz -C {root}/comics/fixture",
        f"mv {root}/staging/compose.yaml {root}/compose.yaml",
        f"docker compose -f {root}/compose.yaml up -d --no-build "
        f"--force-recreate --remove-orphans",
        f"mounts=\"$(docker inspect {container} --format "
        f"'{{{{range .Mounts}}}}{{{{println .Type \"|\" .Source \"|\" "
        f".Destination}}}}{{{{end}}}}')\"",
        f"printf '%s\\n' \"$mounts\" | grep -F 'bind | "
        f"{plan.remote_root}/data | /data'",
        f"printf '%s\\n' \"$mounts\" | grep -F 'bind | "
        f"{plan.remote_root}/cache | /app/.cache'",
        f"printf '%s\\n' \"$mounts\" | grep -F 'bind | "
        f"{plan.remote_root}/comics | /app/comics'",
        f"printf '%s\\n' \"$mounts\" | grep -F 'bind | "
        f"{plan.remote_root}/novels | /app/novels'",
        "if printf '%s\\n' \"$mounts\" | grep -Eq '(^|[ |])/vol1(/|[ |])'; "
        "then echo 'forbidden vol1 mount detected' >&2; exit 1; fi",
        f"test \"$(docker inspect {container} --format "
        f"'{{{{(index (index .NetworkSettings.Ports "
        f"\"3000/tcp\") 0).HostPort}}}}')\" = 17680",
    )
    # All removal targets are literal descendants of the fixed isolated root,
    # except removal of the exact isolated container name.
    return RemoteCommandPlan(commands=commands, destructive=False)


def _excluded(relative: Path) -> bool:
    blocked_parts = {
        ".git",
        ".tmp",
        "node_modules",
        "__pycache__",
        ".next",
        "dist",
    }
    if any(part in blocked_parts for part in relative.parts):
        return True
    name = relative.name.lower()
    if name == ".env" or name.startswith(".env."):
        return True
    if name in {"id_rsa", "id_ed25519"}:
        return True
    return relative.suffix.lower() in {".key", ".pem", ".p12", ".pfx"}


def create_source_archive(source_root: Path, output: Path) -> None:
    source_root = source_root.resolve()
    with tarfile.open(output, "w:gz", compresslevel=6) as archive:
        for path in sorted(source_root.rglob("*")):
            relative = path.relative_to(source_root)
            if _excluded(relative) or path.is_symlink():
                continue
            archive.add(path, arcname=relative.as_posix(), recursive=False)


def create_fixture_archive(fixture_root: Path, output: Path) -> None:
    with tarfile.open(output, "w:gz", compresslevel=6) as archive:
        for path in sorted(fixture_root.rglob("*")):
            relative = path.relative_to(fixture_root)
            archive.add(path, arcname=relative.as_posix(), recursive=False)


def run_checked(command: list[str], input_bytes: bytes | None = None) -> None:
    print("+", subprocess.list2cmdline(command))
    subprocess.run(command, input=input_bytes, check=True)


def ssh_command(plan: DeploymentPlan, remote_command: str) -> list[str]:
    return [
        "ssh",
        "-p",
        str(plan.ssh_port),
        plan.ssh_destination,
        remote_command,
    ]


def scp_command(plan: DeploymentPlan, local_path: Path, remote_path: str) -> list[str]:
    return [
        "scp",
        "-P",
        str(plan.ssh_port),
        str(local_path),
        f"{plan.ssh_destination}:{remote_path}",
    ]


class HTTPClient:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.cookies = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(self.cookies)
        )

    def json(
        self,
        path: str,
        method: str = "GET",
        body: dict | None = None,
    ) -> tuple[int, object]:
        headers = {}
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        request = urllib.request.Request(
            self.base_url + path,
            method=method,
            headers=headers,
            data=data,
        )
        try:
            with self.opener.open(request, timeout=120) as response:
                payload = response.read()
                status = response.status
        except urllib.error.HTTPError as error:
            payload = error.read()
            status = error.code
        try:
            return status, json.loads(payload.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError):
            return status, {}


def wait_for_health(client: HTTPClient, timeout: int = 300) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        status, _ = client.json("/api/health")
        if status == 200:
            return
        time.sleep(2)
    raise TimeoutError("container did not become healthy before timeout")


def ensure_admin_and_fixture_libraries(
    plan: DeploymentPlan,
    manifest: dict,
) -> dict[str, str]:
    if not plan.admin_password:
        raise RuntimeError("administrator password is required for deployment")
    client = HTTPClient(plan.base_url)
    wait_for_health(client)
    status, login = client.json(
        "/api/auth/login",
        "POST",
        {"username": plan.admin_username, "password": plan.admin_password},
    )
    if status != 200:
        status, register = client.json(
            "/api/auth/register",
            "POST",
            {
                "username": plan.admin_username,
                "password": plan.admin_password,
                "nickname": plan.admin_username,
            },
        )
        if status != 200:
            raise RuntimeError(
                f"admin login/register failed: login={login!r}, register={register!r}"
            )

    status, libraries = client.json("/api/admin/libraries")
    if status != 200 or not isinstance(libraries, dict):
        raise RuntimeError(f"cannot list libraries: HTTP {status} {libraries!r}")
    existing_by_root: dict[str, str] = {}
    for library in libraries.get("libraries", []):
        roots = library.get("rootPaths") or [library.get("rootPath")]
        for root in roots:
            if root:
                existing_by_root[str(root)] = str(library.get("id", ""))

    library_ids: dict[str, str] = {}
    variants = manifest.get("libraries", [])
    if len(variants) != 7:
        raise RuntimeError("parallel fixture manifest must contain seven libraries")
    for variant in variants:
        code = str(variant.get("code", ""))
        root_path = str(variant.get("rootPath", ""))
        library_id = existing_by_root.get(root_path, "")
        if not library_id:
            status, created = client.json(
                "/api/admin/libraries",
                "POST",
                {
                    "name": str(variant.get("libraryName", f"{plan.library_name} {code}")),
                    "type": "comic",
                    "rootPath": root_path,
                    "rootPaths": [root_path],
                    "enabled": True,
                    "defaultAccess": "private",
                    "scanEnabled": True,
                },
            )
            if status != 201 or not isinstance(created, dict):
                raise RuntimeError(
                    f"cannot create fixture library {code}: {status} {created!r}"
                )
            library_id = str(created.get("library", {}).get("id", ""))
        if not code or not library_id:
            raise RuntimeError(f"fixture library {code or root_path} has no ID")
        library_ids[code] = library_id

    for code, library_id in library_ids.items():
        status, response = client.json(
            f"/api/admin/libraries/{urllib.parse.quote(library_id, safe='')}/scan",
            "POST",
            {},
        )
        if status not in (200, 202):
            raise RuntimeError(
                f"fixture scan {code} failed: HTTP {status} {response!r}"
            )
    return library_ids


def deploy(plan: DeploymentPlan, source_root: Path) -> None:
    validate_plan(plan)
    with tempfile.TemporaryDirectory(prefix="ymreader-deploy-") as temp_name:
        temp = Path(temp_name)
        fixture_root = temp / "fixture"
        manifest = build_fixture(fixture_root)
        source_archive = temp / "source.tar.gz"
        fixture_archive = temp / "fixture.tar.gz"
        compose_path = temp / "compose.yaml"
        create_source_archive(source_root, source_archive)
        create_fixture_archive(fixture_root, fixture_archive)
        compose_path.write_text(render_compose(plan), encoding="utf-8", newline="\n")

        root = plan.remote_root
        run_checked(
            ssh_command(
                plan,
                f"test {shell_quote(root)} = /vol3/1000/ymreader-unified-work-clean "
                f"&& mkdir -p {shell_quote(root)}/staging",
            )
        )
        for local, remote_name in (
            (source_archive, "source.tar.gz"),
            (fixture_archive, "fixture.tar.gz"),
            (compose_path, "compose.yaml"),
        ):
            run_checked(
                scp_command(plan, local, f"{root}/staging/{remote_name}")
            )
        commands = build_remote_commands(plan, execute=True)
        remote_script = "set -eu\n" + "\n".join(commands.commands)
        run_checked(ssh_command(plan, "sh -s"), remote_script.encode("utf-8"))
    library_ids = ensure_admin_and_fixture_libraries(plan, manifest)
    print(
        "parallel fixture libraries ready: "
        + ", ".join(f"{code}={library_id}" for code, library_id in library_ids.items())
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--execute", action="store_true")
    parser.add_argument(
        "--confirm-target",
        default="",
        help=f"required with --execute: {CONFIRMATION_TOKEN}",
    )
    parser.add_argument("--source-root", type=Path, default=Path(__file__).parents[1])
    parser.add_argument("--ssh-user", default="ymzwh")
    parser.add_argument("--ssh-port", type=int, default=22)
    parser.add_argument("--puid", type=int, default=1000)
    parser.add_argument("--pgid", type=int, default=1000)
    parser.add_argument("--admin-username", default="admin")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    plan = DeploymentPlan(
        ssh_user=args.ssh_user,
        ssh_port=args.ssh_port,
        puid=args.puid,
        pgid=args.pgid,
        admin_username=args.admin_username,
        admin_password=resolve_admin_password(args.execute),
    )
    validate_plan(plan)
    print(render_compose(plan))
    print("Remote command plan:")
    for command in build_remote_commands(plan).commands:
        print("  ", command)
    if not args.execute:
        print("DRY RUN ONLY: no network, filesystem, Docker, or NAS changes were made.")
        return 0
    if args.confirm_target != CONFIRMATION_TOKEN:
        raise SystemExit(
            "--execute requires --confirm-target "
            f"{CONFIRMATION_TOKEN}"
        )
    deploy(plan, args.source_root.resolve())
    print(f"deployed isolated test container at {plan.base_url}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
