#!/usr/bin/env python3
"""End-to-end acceptance checks for the isolated unified-work container."""

from __future__ import annotations

import argparse
import getpass
import hashlib
import http.cookiejar
import io
import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
import xml.etree.ElementTree as ET
from pathlib import Path

from PIL import Image


SAFE_DEFAULT_BASE_URL = "http://192.168.2.9:17680"
PRODUCTION_PORT = 6680
ATOM = {
    "atom": "http://www.w3.org/2005/Atom",
    "dcterms": "http://purl.org/dc/terms/",
}
WORK_STATS_PATH = "/api/stats?view=work&includeNovels=true"


def work_stats_path() -> str:
    return WORK_STATS_PATH


def expected_work_count_after_read(
    baseline_total: int,
    work_was_already_read: bool,
) -> int:
    return baseline_total if work_was_already_read else baseline_total + 1


def opds_work_detail_path(work_id: str) -> str:
    return f"/api/opds/works/{work_id}?page=1&pageSize=500"


def validate_target_url(value: str) -> str:
    parsed = urllib.parse.urlsplit(value.rstrip("/"))
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise ValueError("base URL must be an absolute HTTP(S) URL")
    if parsed.username or parsed.password:
        raise ValueError("credentials are not allowed in the base URL")
    if parsed.hostname != "192.168.2.9":
        raise ValueError("acceptance checks are locked to NAS 192.168.2.9")
    try:
        port = parsed.port
    except ValueError as error:
        raise ValueError("base URL contains an invalid port") from error
    if port == PRODUCTION_PORT:
        raise ValueError("production port 6680 is permanently forbidden")
    if port != 17680:
        raise ValueError("acceptance checks are locked to isolated port 17680")
    if parsed.query or parsed.fragment:
        raise ValueError("base URL cannot contain a query or fragment")
    path = parsed.path.rstrip("/")
    return urllib.parse.urlunsplit(
        (parsed.scheme, parsed.netloc, path, "", "")
    )


def resolve_admin_password() -> str:
    password = os.environ.get("YMREADER_ADMIN_PASSWORD", "")
    if password:
        return password
    if not sys.stdin.isatty():
        raise RuntimeError(
            "YMREADER_ADMIN_PASSWORD is required when no interactive terminal is available"
        )
    password = getpass.getpass("YMReader test administrator password: ")
    if not password:
        raise RuntimeError("administrator password cannot be empty")
    return password


class Client:
    def __init__(self, base_url: str) -> None:
        self.base_url = validate_target_url(base_url)
        self.cookies = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(self.cookies)
        )

    def request(
        self,
        path: str,
        method: str = "GET",
        body: dict | None = None,
        headers: dict[str, str] | None = None,
    ) -> tuple[int, object, bytes]:
        data = None
        request_headers = dict(headers or {})
        if body is not None:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
            request_headers["Content-Type"] = "application/json"
        request = urllib.request.Request(
            self.base_url + path,
            data=data,
            headers=request_headers,
            method=method,
        )
        try:
            with self.opener.open(request, timeout=120) as response:
                return response.status, response.headers, response.read()
        except urllib.error.HTTPError as error:
            return error.code, error.headers, error.read()

    def json(
        self,
        path: str,
        method: str = "GET",
        body: dict | None = None,
    ) -> tuple[int, object]:
        status, _, payload = self.request(path, method, body)
        try:
            decoded = json.loads(payload.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError):
            decoded = None
        return status, decoded


def payload_items(payload: object, *keys: str) -> list[dict]:
    if isinstance(payload, list):
        return [item for item in payload if isinstance(item, dict)]
    if isinstance(payload, dict):
        for key in keys:
            value = payload.get(key)
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
    return []


def parse_feed(payload: bytes) -> tuple[ET.Element | None, list[ET.Element]]:
    try:
        root = ET.fromstring(payload)
    except ET.ParseError:
        return None, []
    return root, root.findall("atom:entry", ATOM)


def feed_titles(entries: list[ET.Element]) -> list[str]:
    result = []
    for entry in entries:
        title = entry.find("atom:title", ATOM)
        result.append((title.text or "").strip() if title is not None else "")
    return result


def feed_entry_categories(entry: ET.Element) -> set[str]:
    return {
        str(category.attrib.get("term", "")).strip()
        for category in entry.findall("atom:category", ATOM)
        if str(category.attrib.get("term", "")).strip()
    }


def sha256(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def image_fingerprint(payload: bytes) -> dict:
    with Image.open(io.BytesIO(payload)) as image:
        image.verify()
    with Image.open(io.BytesIO(payload)) as image:
        width, height = image.size
        grayscale = image.convert("L").resize((8, 8))
        pixels = list(grayscale.get_flattened_data())
    average = sum(pixels) / len(pixels)
    bits = "".join("1" if pixel >= average else "0" for pixel in pixels)
    return {
        "sha256": sha256(payload),
        "width": width,
        "height": height,
        "aspectRatio": width / height if height else 0,
        "averageHash": f"{int(bits, 2):016x}",
    }


def readable_image_fingerprint(payload: bytes) -> dict | None:
    if not payload:
        return None
    try:
        return image_fingerprint(payload)
    except (OSError, ValueError, SyntaxError):
        return None


def hash_distance(left: str, right: str) -> int:
    try:
        return (int(left, 16) ^ int(right, 16)).bit_count()
    except (TypeError, ValueError):
        return 65


def acquisition_mime_for_path(value: str) -> str:
    suffix = Path(value).suffix.lower()
    if suffix in {".cbz", ".zip"}:
        return "application/vnd.comicbook+zip"
    if suffix == ".pdf":
        return "application/pdf"
    return ""


class VerificationSuite:
    def __init__(
        self,
        base_url: str,
        username: str,
        password: str,
        manifest: dict | None = None,
        exercise_state: bool = True,
        comparison_report_path: Path | None = None,
    ) -> None:
        self.base_url = validate_target_url(base_url)
        self.client = Client(self.base_url)
        self.username = username
        self.password = password
        self.manifest = manifest or {}
        self.exercise_state = exercise_state
        self.failures: list[str] = []
        self.passes = 0
        self.works: list[dict] = []
        self.details: dict[str, dict] = {}
        self.comparison_report_path = comparison_report_path
        self.comparison_report: dict = {
            "schemaVersion": 1,
            "baseUrl": self.base_url,
            "status": "not-run",
            "allowedDifferences": self.manifest.get("comparison", {}).get(
                "allowedDifferencePaths",
                [],
            ),
            "variants": [],
            "violations": [],
        }

    def check(self, condition: bool, message: str) -> None:
        if condition:
            self.passes += 1
            print(f"PASS {message}")
        else:
            self.failures.append(message)
            print(f"FAIL {message}")

    def check_status(
        self,
        path: str,
        expected: set[int],
        message: str,
        client: Client | None = None,
    ) -> tuple[int, object]:
        status, payload = (client or self.client).json(path)
        self.check(status in expected, f"{message} (HTTP {status})")
        return status, payload

    def verify_auth_and_permissions(self) -> None:
        anonymous = Client(self.base_url)
        for path in ("/api/works", "/api/stats", "/api/admin/libraries"):
            status, _ = anonymous.json(path)
            self.check(
                status in {401, 403},
                f"anonymous access is denied for {path} (HTTP {status})",
            )

        status, health = self.client.json("/api/health")
        self.check(status == 200, "health endpoint returns 200")
        self.check(isinstance(health, dict), "health endpoint returns JSON")

        status, login = self.client.json(
            "/api/auth/login",
            "POST",
            {"username": self.username, "password": self.password},
        )
        self.check(status == 200, "administrator login succeeds")
        self.check(isinstance(login, dict), "login returns JSON")
        status, me = self.client.json("/api/auth/me")
        user = me.get("user") if isinstance(me, dict) else None
        self.check(status == 200, "authenticated /api/auth/me succeeds")
        self.check(
            isinstance(user, dict) and user.get("role") == "admin",
            "test account has administrator role",
        )
        status, _ = self.client.json("/api/admin/libraries")
        self.check(status == 200, "administrator can access library management")

    def load_works(self) -> None:
        expected = int(self.manifest.get("expectedWorks", 4))
        status = 0
        payload: object = None
        attempts = 60 if self.manifest.get("libraries") else 1
        for _ in range(attempts):
            status, payload = self.client.json(
                "/api/works?page=1&pageSize=500"
            )
            self.works = payload_items(payload, "works")
            if status == 200 and (
                not self.manifest or len(self.works) >= expected
            ):
                break
            time.sleep(2)
        self.check(status == 200, "GET /api/works returns 200")
        if self.manifest:
            self.check(
                len(self.works) == expected,
                f"work list contains exactly {expected} fixture Works",
            )
        else:
            self.check(
                len(self.works) >= expected,
                f"work list contains at least {expected} Works",
            )
        ids = [str(work.get("id", "")) for work in self.works]
        self.check(len(set(ids)) == len(ids), "Work IDs are unique")
        self.check(
            all(value.startswith("work_") for value in ids),
            "all logical Works use stable work_ IDs",
        )
        status, repeated = self.client.json("/api/works?page=1&pageSize=500")
        repeated_ids = [
            str(item.get("id", ""))
            for item in payload_items(repeated, "works")
        ]
        self.check(
            status == 200 and repeated_ids == ids,
            "Work IDs and ordering are stable across repeated reads",
        )

    def verify_work_unit_page_model(self) -> None:
        expected_by_title = {
            str(item.get("title")): item
            for item in self.manifest.get("works", [])
            if isinstance(item, dict)
        }
        actual_by_title = {
            str(item.get("title")): item for item in self.works
        }
        if expected_by_title:
            self.check(
                set(expected_by_title) == set(actual_by_title),
                "all fixture formats resolve to the expected Work titles",
            )

        for work in self.works:
            title = str(work.get("title", ""))
            work_id = urllib.parse.quote(str(work.get("id", "")), safe="")
            status, detail = self.client.json(f"/api/works/{work_id}")
            self.check(status == 200, f"Work detail opens: {title}")
            if not isinstance(detail, dict):
                continue
            self.details[str(work.get("id"))] = detail
            units = payload_items(detail, "units")
            status, unit_payload = self.client.json(
                f"/api/works/{work_id}/units"
            )
            listed_units = payload_items(unit_payload, "units")
            self.check(
                status == 200 and [u.get("id") for u in listed_units]
                == [u.get("id") for u in units],
                f"Work unit endpoint matches detail: {title}",
            )
            self.check(bool(units), f"Work exposes at least one Unit: {title}")
            self.check(
                all(unit.get("workId") == work.get("id") for unit in units),
                f"all Units point back to their Work: {title}",
            )
            self.check(
                [int(unit.get("sortIndex", 0) or 0) for unit in units]
                == sorted(int(unit.get("sortIndex", 0) or 0) for unit in units),
                f"Units are naturally ordered: {title}",
            )
            expected = expected_by_title.get(title, {})
            if expected:
                self.check(
                    len(units) == expected.get("expectedUnits"),
                    f"{expected.get('format')} has expected Unit count",
                )
                physical_ids = {
                    str(unit.get("comicId", "")) for unit in units
                }
                self.check(
                    len(physical_ids) == expected.get("expectedPhysicalComics"),
                    f"{expected.get('format')} has expected physical Comic count",
                )
                has_internal = any(unit.get("internalPath") for unit in units)
                self.check(
                    has_internal == bool(expected.get("expectInternalUnits")),
                    f"{expected.get('format')} internal Unit detection is correct",
                )

            self.check(
                bool(detail.get("coverComicId") or detail.get("coverUrl")),
                f"Work has a cover source: {title}",
            )
            self.check(
                detail.get("metadataHostType") in {"series", "comic"}
                and bool(detail.get("metadataHostId")),
                f"Work has a stable metadata host: {title}",
            )
            cover_status, cover_headers, cover = self.client.request(
                f"/api/opds/work-cover/{work_id}"
            )
            cover_fingerprint = readable_image_fingerprint(cover)
            self.check(
                cover_status == 200
                and cover_fingerprint is not None
                and str(cover_headers.get("Content-Type", "")).startswith("image/"),
                f"Work cover is readable: {title}",
            )

            unit_cover_hashes = []
            for unit in units:
                unit_label = str(
                    unit.get("displayLabel") or unit.get("title") or unit.get("id")
                )
                comic_id = urllib.parse.quote(
                    str(unit.get("comicId", "")),
                    safe="",
                )
                status, comic = self.client.json(f"/api/comics/{comic_id}")
                self.check(
                    status == 200 and isinstance(comic, dict),
                    f"physical Comic backing Unit opens: {title} / {unit_label}",
                )
                status, pages = self.client.json(f"/api/comics/{comic_id}/pages")
                page_items = payload_items(pages, "pages")
                total_pages = (
                    int(pages.get("totalPages", 0))
                    if isinstance(pages, dict)
                    else 0
                )
                start_page = int(unit.get("startPage", 0) or 0)
                unit_pages = int(unit.get("pageCount", 0) or 0)
                self.check(
                    status == 200
                    and total_pages > start_page
                    and len(page_items) == total_pages,
                    f"Unit resolves Page list: {title} / {unit_label}",
                )
                if unit_pages > 0:
                    self.check(
                        start_page + unit_pages <= total_pages,
                        f"Unit Page window is within physical Comic: "
                        f"{title} / {unit_label}",
                    )
                page_status, page_headers, page = self.client.request(
                    f"/api/comics/{comic_id}/page/{start_page}"
                )
                self.check(
                    page_status == 200
                    and len(page) > 0
                    and str(page_headers.get("Content-Type", "")).startswith(
                        "image/"
                    ),
                    f"first Page renders: {title} / {unit_label}",
                )

                if unit.get("internalPath"):
                    cover_page = int(
                        unit.get("coverPage", start_page) or start_page
                    )
                    cover_path = (
                        f"/api/opds/unit-cover/{comic_id}?page={cover_page}"
                    )
                else:
                    cover_path = f"/api/comics/{comic_id}/thumbnail"
                unit_status, unit_headers, unit_cover = self.client.request(
                    cover_path
                )
                self.check(
                    unit_status == 200
                    and len(unit_cover) > 0
                    and str(unit_headers.get("Content-Type", "")).startswith(
                        "image/"
                    ),
                    f"Unit cover is readable: {title} / {unit_label}",
                )
                if unit_status == 200 and unit_cover:
                    unit_cover_hashes.append(sha256(unit_cover))

                if unit.get("internalPath") and unit_pages > 0:
                    query = urllib.parse.urlencode(
                        {
                            "page": 0,
                            "width": 0,
                            "startPage": start_page,
                            "pageCount": unit_pages,
                        }
                    )
                    stream_status, _, stream_page = self.client.request(
                        f"/api/opds/stream/{comic_id}?{query}"
                    )
                    self.check(
                        stream_status == 200 and len(stream_page) > 0,
                        f"virtual archive Unit streams independently: "
                        f"{title} / {unit_label}",
                    )

            if expected.get("expectDistinctUnitCovers") and len(units) > 1:
                self.check(
                    len(unit_cover_hashes) == len(units)
                    and len(set(unit_cover_hashes)) == len(units),
                    f"Units retain distinct covers: {title}",
                )

            if expected.get("expectPdf"):
                self.check(
                    len(units) == 1
                    and str(units[0].get("relativePath", "")).lower().endswith(
                        ".pdf"
                    ),
                    f"PDF is represented as one readable Unit: {title}",
                )

    def verify_series_view_and_filters(self) -> None:
        status, shelf = self.client.json(
            "/api/comics?seriesView=true&contentType=comic&page=1&pageSize=500"
        )
        shelf_items = payload_items(shelf, "comics", "works")
        self.check(status == 200, "seriesView shelf endpoint returns 200")
        self.check(
            len(shelf_items) == len(self.works),
            "seriesView returns one card per Work",
        )
        self.check(
            all(str(item.get("id", "")).startswith("work_") for item in shelf_items),
            "seriesView contains Work cards rather than physical Units",
        )
        if not self.works:
            return
        title = str(self.works[0].get("title", ""))
        first_library_id = str(self.works[0].get("libraryId", ""))
        query = urllib.parse.urlencode(
            {
                "seriesView": "true",
                "contentType": "comic",
                "search": title,
                "libraryIds": first_library_id,
                "page": 1,
                "pageSize": 50,
            }
        )
        status, searched = self.client.json(f"/api/comics?{query}")
        searched_items = payload_items(searched, "comics", "works")
        self.check(
            status == 200
            and len(searched_items) == 1
            and searched_items[0].get("id") == self.works[0].get("id"),
            "seriesView search filters at Work level",
        )
        library_id = str(self.works[0].get("libraryId", ""))
        if library_id:
            query = urllib.parse.urlencode(
                {
                    "seriesView": "true",
                    "contentType": "comic",
                    "libraryIds": library_id,
                    "page": 1,
                    "pageSize": 500,
                }
            )
            status, filtered = self.client.json(f"/api/comics?{query}")
            filtered_items = payload_items(filtered, "comics", "works")
            self.check(
                status == 200
                and filtered_items
                and all(item.get("libraryId") == library_id for item in filtered_items),
                "seriesView library filter operates at Work level",
            )
        for path, label in (
            ("/api/tags", "tag management list"),
            ("/api/categories", "category management list"),
            ("/api/recommendations", "recommendations"),
            ("/api/stats", "reading statistics"),
            ("/api/stats/enhanced", "enhanced reading statistics"),
        ):
            status, payload = self.client.json(path)
            self.check(
                status == 200 and payload is not None,
                f"{label} endpoint remains available",
            )

    def verify_work_metadata_filters(self) -> tuple[str, str] | None:
        if not self.exercise_state:
            return None
        target = next(
            (
                work
                for work in self.works
                if work.get("metadataHostType") == "comic"
                and work.get("metadataHostId")
            ),
            None,
        )
        self.check(
            target is not None,
            "fixture contains a Work with a writable Comic metadata host",
        )
        if target is None:
            return None

        work_id = str(target.get("id"))
        title = str(target.get("title", ""))
        host_id = urllib.parse.quote(str(target.get("metadataHostId")), safe="")
        unique = uuid.uuid4().hex[:12]
        tag_name = f"e2e-tag-{unique}"
        category_name = f"E2E Category {unique}"
        category_slug = f"e2e-category-{unique}"

        status, created = self.client.json(
            "/api/categories/create",
            "POST",
            {
                "name": category_name,
                "slug": category_slug,
                "icon": "book",
            },
        )
        created_category = (
            created.get("category") if isinstance(created, dict) else None
        )
        self.check(
            status == 200
            and isinstance(created_category, dict)
            and created_category.get("slug") == category_slug,
            "unique acceptance category is created",
        )
        status, _ = self.client.json(
            f"/api/comics/{host_id}/tags",
            "POST",
            {"tags": [tag_name]},
        )
        self.check(
            status == 200,
            "unique acceptance tag is bound to the Work metadata host",
        )
        status, _ = self.client.json(
            f"/api/comics/{host_id}/categories",
            "POST",
            {"categorySlugs": [category_slug]},
        )
        self.check(
            status == 200,
            "unique acceptance category is bound to the Work metadata host",
        )
        time.sleep(0.2)

        status, all_tags = self.client.json("/api/tags")
        listed_tags = {
            str(item.get("name", ""))
            for item in payload_items(all_tags, "tags")
        }
        self.check(
            status == 200 and tag_name in listed_tags,
            "unique acceptance tag exists in tag management",
        )
        status, all_categories = self.client.json("/api/categories")
        listed_categories = {
            str(item.get("slug", ""))
            for item in payload_items(all_categories, "categories")
        }
        self.check(
            status == 200 and category_slug in listed_categories,
            "unique acceptance category exists in category management",
        )

        status, detail = self.client.json(
            f"/api/works/{urllib.parse.quote(work_id, safe='')}"
        )
        if isinstance(detail, dict):
            self.details[work_id] = detail
        tags = {
            str(item.get("name", ""))
            for item in payload_items(detail, "tags")
        }
        categories = payload_items(detail, "categories")
        category_slugs = {str(item.get("slug", "")) for item in categories}
        self.check(
            status == 200 and tag_name in tags,
            "Work detail inherits the metadata-host tag",
        )
        self.check(
            status == 200 and category_slug in category_slugs,
            "Work detail inherits the metadata-host category",
        )

        for parameter, value, label in (
            ("tags", tag_name, "tag"),
            ("category", category_slug, "category"),
        ):
            query = urllib.parse.urlencode(
                {
                    parameter: value,
                    "page": 1,
                    "pageSize": 50,
                }
            )
            status, filtered = self.client.json(f"/api/works?{query}")
            items = payload_items(filtered, "works")
            self.check(
                status == 200
                and len(items) == 1
                and items[0].get("id") == work_id,
                f"/api/works {label} filter returns one Work without Unit duplicates",
            )

            series_query = urllib.parse.urlencode(
                {
                    "seriesView": "true",
                    "contentType": "comic",
                    parameter: value,
                    "page": 1,
                    "pageSize": 50,
                }
            )
            status, filtered = self.client.json(
                f"/api/comics?{series_query}"
            )
            items = payload_items(filtered, "comics", "works")
            self.check(
                status == 200
                and len(items) == 1
                and items[0].get("id") == work_id,
                f"seriesView {label} filter returns one Work without Unit duplicates",
            )

        search_query = urllib.parse.urlencode(
            {
                "search": title,
                "libraryIds": target.get("libraryId", ""),
                "page": 1,
                "pageSize": 50,
            }
        )
        status, searched = self.client.json(f"/api/works?{search_query}")
        search_items = payload_items(searched, "works")
        searched_tags = (
            {
                str(item.get("name", ""))
                for item in payload_items(search_items[0], "tags")
            }
            if len(search_items) == 1
            else set()
        )
        searched_categories = (
            {
                str(item.get("slug", ""))
                for item in payload_items(search_items[0], "categories")
            }
            if len(search_items) == 1
            else set()
        )
        self.check(
            status == 200
            and len(search_items) == 1
            and search_items[0].get("id") == work_id,
            "Work search returns one Work instead of matching Units",
        )
        self.check(
            tag_name in searched_tags and category_slug in searched_categories,
            "Work search preserves tag and category metadata",
        )

        opds_query = urllib.parse.urlencode(
            {
                "q": title,
                "libraryIds": target.get("libraryId", ""),
                "page": 1,
                "pageSize": 50,
            }
        )
        status, _, payload = self.client.request(
            f"/api/opds/search?{opds_query}"
        )
        root, entries = parse_feed(payload)
        opds_categories = (
            feed_entry_categories(entries[0]) if len(entries) == 1 else set()
        )
        self.check(
            status == 200
            and root is not None
            and len(entries) == 1
            and feed_titles(entries)[0] == title,
            "OPDS search returns one Work without Unit duplicates",
        )
        self.check(
            tag_name in opds_categories and category_name in opds_categories,
            "OPDS search metadata matches the Work tag and category",
        )
        return tag_name, category_slug

    def cleanup_work_metadata(self, state: tuple[str, str] | None) -> None:
        if state is None:
            return
        tag_name, category_slug = state
        self.client.json("/api/tags", "DELETE", {"name": tag_name})
        self.client.json(
            f"/api/categories/{urllib.parse.quote(category_slug, safe='')}",
            "DELETE",
            {},
        )

    def _comparison_violation(
        self,
        variant_report: dict,
        message: str,
        expected: object = None,
        actual: object = None,
    ) -> None:
        item = {"message": message, "expected": expected, "actual": actual}
        variant_report["violations"].append(item)
        self.comparison_report["violations"].append(
            {"variant": variant_report.get("code"), **item}
        )
        self.check(False, f"{variant_report.get('code')}: {message}")

    def _wait_for_library_work(
        self,
        library_id: str,
        retries: int = 60,
    ) -> list[dict]:
        query = urllib.parse.urlencode(
            {"libraryIds": library_id, "page": 1, "pageSize": 50}
        )
        for _ in range(retries):
            status, payload = self.client.json(f"/api/works?{query}")
            works = payload_items(payload, "works")
            if status == 200 and works:
                return works
            time.sleep(2)
        return []

    def _comparison_opds_snapshot(
        self,
        work: dict,
        library_id: str,
    ) -> dict:
        title = str(work.get("title", ""))
        search_query = urllib.parse.urlencode(
            {
                "q": title,
                "libraryIds": library_id,
                "page": 1,
                "pageSize": 50,
            }
        )
        search_status, _, search_payload = self.client.request(
            f"/api/opds/search?{search_query}"
        )
        search_root, search_entries = parse_feed(search_payload)

        work_id = urllib.parse.quote(str(work.get("id", "")), safe="")
        detail_status, _, detail_payload = self.client.request(
            f"/api/opds/works/{work_id}?page=1&pageSize=50"
        )
        detail_root, detail_entries = parse_feed(detail_payload)
        acquisition_mimes = []
        pse_links = 0
        unit_page_counts = []
        for entry in detail_entries:
            extent = entry.find("dcterms:extent", ATOM)
            match = re.search(
                r"(\d+)",
                extent.text if extent is not None and extent.text else "",
            )
            unit_page_counts.append(int(match.group(1)) if match else 0)
            for link in entry.findall("atom:link", ATOM):
                rel = str(link.attrib.get("rel", ""))
                mime = str(link.attrib.get("type", ""))
                if "acquisition" in rel and mime:
                    acquisition_mimes.append(mime)
                if "opds-pse/stream" in rel:
                    pse_links += 1
        return {
            "searchStatus": search_status,
            "searchValid": search_root is not None,
            "searchTitles": feed_titles(search_entries),
            "searchCategories": (
                sorted(feed_entry_categories(search_entries[0]))
                if len(search_entries) == 1
                else []
            ),
            "detailStatus": detail_status,
            "detailValid": detail_root is not None,
            "unitTitles": feed_titles(detail_entries),
            "unitPageCounts": unit_page_counts,
            "acquisitionMimes": sorted(set(acquisition_mimes)),
            "pseLinkCount": pse_links,
        }

    def _comparison_reading_snapshot(
        self,
        work: dict,
        units: list[dict],
        library_id: str,
        variant_report: dict,
    ) -> dict:
        if not self.exercise_state or not units:
            return {"skipped": True}
        status, baseline = self.client.json(work_stats_path())
        baseline_sessions = (
            int(baseline.get("totalSessions", 0))
            if status == 200 and isinstance(baseline, dict)
            else 0
        )
        baseline_works = (
            int(baseline.get("totalComicsRead", 0))
            if status == 200 and isinstance(baseline, dict)
            else 0
        )

        positions = [
            (units[0], 0),
            (units[-1], max(0, int(units[-1].get("pageCount", 0) or 0) - 1)),
        ]
        session_ids = []
        checkpoint_global_pages = []
        for index, (unit, relative_page) in enumerate(positions):
            raw_comic_id = str(unit.get("comicId", ""))
            comic_id = urllib.parse.quote(raw_comic_id, safe="")
            absolute_page = int(unit.get("startPage", 0) or 0) + relative_page
            unit_index = units.index(unit)
            checkpoint_global_pages.append(
                sum(
                    int(previous.get("pageCount", 0) or 0)
                    for previous in units[:unit_index]
                )
                + relative_page
            )
            pages_status, pages = self.client.json(
                f"/api/comics/{comic_id}/pages"
            )
            total_pages = (
                int(pages.get("totalPages", 0))
                if isinstance(pages, dict)
                else 0
            )
            if pages_status != 200 or total_pages <= absolute_page:
                self._comparison_violation(
                    variant_report,
                    f"reading checkpoint {index + 1} is not readable",
                    f"totalPages > {absolute_page}",
                    total_pages,
                )
                continue
            progress_status, _ = self.client.json(
                f"/api/comics/{comic_id}/progress",
                "PUT",
                {"page": absolute_page, "totalPages": total_pages},
            )
            if progress_status != 200:
                self._comparison_violation(
                    variant_report,
                    f"reading checkpoint {index + 1} progress update failed",
                    200,
                    progress_status,
                )
            session_status, session = self.client.json(
                "/api/stats/session",
                "POST",
                {"comicId": raw_comic_id, "startPage": absolute_page},
            )
            session_id = (
                int(session.get("sessionId", 0))
                if isinstance(session, dict)
                else 0
            )
            if session_status == 200 and session_id:
                end_status, _ = self.client.json(
                    "/api/stats/session",
                    "PUT",
                    {
                        "sessionId": session_id,
                        "endPage": absolute_page,
                        "duration": 1,
                    },
                )
                if end_status == 200:
                    session_ids.append(session_id)
            if index == 0:
                time.sleep(1.05)

        work_id = str(work.get("id", ""))
        status, refreshed = self.client.json(
            f"/api/works/{urllib.parse.quote(work_id, safe='')}"
        )
        refreshed_units = payload_items(refreshed, "units")
        continue_unit_id = (
            str(refreshed.get("continueUnitId", ""))
            if isinstance(refreshed, dict)
            else ""
        )
        continue_index = next(
            (
                index
                for index, unit in enumerate(refreshed_units)
                if str(unit.get("id", "")) == continue_unit_id
            ),
            -1,
        )
        continue_relative_page = -1
        if continue_index >= 0:
            continue_relative_page = int(
                refreshed_units[continue_index].get("lastReadPage", -1)
            )
        continue_global_page = (
            sum(
                int(unit.get("pageCount", 0) or 0)
                for unit in refreshed_units[:continue_index]
            )
            + continue_relative_page
            if continue_index >= 0
            else -1
        )

        history_query = urllib.parse.urlencode(
            {
                "libraryIds": library_id,
                "sortBy": "lastReadAt",
                "sortOrder": "desc",
                "page": 1,
                "pageSize": 50,
            }
        )
        history_status, history_payload = self.client.json(
            f"/api/works?{history_query}"
        )
        history_works = [
            item
            for item in payload_items(history_payload, "works")
            if item.get("lastReadAt")
        ]
        stats_status, stats = self.client.json(work_stats_path())
        total_sessions = (
            int(stats.get("totalSessions", 0))
            if isinstance(stats, dict)
            else -1
        )
        total_works = (
            int(stats.get("totalComicsRead", 0))
            if isinstance(stats, dict)
            else -1
        )
        return {
            "detailStatus": status,
            "continueUnitIndex": continue_index,
            "continueRelativePage": continue_relative_page,
            "continueGlobalPage": continue_global_page,
            "historyStatus": history_status,
            "historyWorkIds": [str(item.get("id", "")) for item in history_works],
            "historyWorkCount": len(history_works),
            "statsStatus": stats_status,
            "sessionIds": session_ids,
            "checkpointGlobalPages": checkpoint_global_pages,
            "sessionDelta": total_sessions - baseline_sessions,
            "workCountDelta": total_works - baseline_works,
        }

    def _comparison_variant_snapshot(
        self,
        variant: dict,
        library_id: str,
        work: dict,
        tag_name: str,
        category_name: str,
        category_slug: str,
        variant_report: dict,
    ) -> dict:
        work_id = str(work.get("id", ""))
        status, detail = self.client.json(
            f"/api/works/{urllib.parse.quote(work_id, safe='')}"
        )
        if status != 200 or not isinstance(detail, dict):
            self._comparison_violation(
                variant_report,
                "Work detail API is unavailable",
                200,
                status,
            )
            return {}
        self.details[work_id] = detail
        units = sorted(
            payload_items(detail, "units"),
            key=lambda unit: int(unit.get("sortIndex", 0) or 0),
        )

        cover_status, _, cover_payload = self.client.request(
            f"/api/opds/work-cover/{urllib.parse.quote(work_id, safe='')}"
        )
        cover = (
            readable_image_fingerprint(cover_payload)
            if cover_status == 200
            else None
        ) or {}
        unit_snapshots = []
        flattened_pages = []
        normalized_cursor = 0
        for unit in units:
            comic_id = urllib.parse.quote(str(unit.get("comicId", "")), safe="")
            page_status, page_payload = self.client.json(
                f"/api/comics/{comic_id}/pages"
            )
            total_pages = (
                int(page_payload.get("totalPages", 0))
                if isinstance(page_payload, dict)
                else 0
            )
            start_page = int(unit.get("startPage", 0) or 0)
            unit_page_count = int(unit.get("pageCount", 0) or 0)
            page_fingerprints = []
            if page_status == 200:
                for page_index in range(
                    start_page,
                    start_page + unit_page_count,
                ):
                    image_status, _, image_payload = self.client.request(
                        f"/api/comics/{comic_id}/page/{page_index}"
                    )
                    if image_status == 200:
                        fingerprint = image_fingerprint(image_payload)
                        page_fingerprints.append(fingerprint)
            normalized_page_fingerprints = list(page_fingerprints)
            if variant.get("code") != "G":
                chapter_title = str(unit.get("displayLabel", ""))
                expected_cover_hash = (
                    self.manifest.get("comparison", {})
                    .get("canonicalPageAverageHash", {})
                    .get(f"{chapter_title}/cover.jpg")
                )
                cover_index = next(
                    (
                        index
                        for index, fingerprint in enumerate(page_fingerprints)
                        if fingerprint.get("averageHash")
                        == expected_cover_hash
                    ),
                    -1,
                )
                if cover_index >= 0:
                    normalized_page_fingerprints = [
                        page_fingerprints[cover_index],
                        *page_fingerprints[:cover_index],
                        *page_fingerprints[cover_index + 1 :],
                    ]
            flattened_pages.extend(normalized_page_fingerprints)
            unit_snapshots.append(
                {
                    "id": unit.get("id"),
                    "comicId": unit.get("comicId"),
                    "title": unit.get("title"),
                    "displayLabel": unit.get("displayLabel"),
                    "relativePath": unit.get("relativePath"),
                    "internalPath": unit.get("internalPath"),
                    "startPage": start_page,
                    "endPage": start_page + unit_page_count - 1,
                    "normalizedStartPage": normalized_cursor,
                    "normalizedEndPage": normalized_cursor + unit_page_count - 1,
                    "pageCount": unit_page_count,
                    "pageListTotal": total_pages,
                    "rawPageFingerprints": page_fingerprints,
                    "pageFingerprints": normalized_page_fingerprints,
                }
            )
            normalized_cursor += unit_page_count

        frontend_status, _, frontend_payload = self.client.request(
            f"/work/{urllib.parse.quote(work_id, safe='')}"
        )
        search_query = urllib.parse.urlencode(
            {
                "libraryIds": library_id,
                "search": detail.get("title", ""),
                "page": 1,
                "pageSize": 50,
            }
        )
        search_status, search_payload = self.client.json(
            f"/api/works?{search_query}"
        )
        search_works = payload_items(search_payload, "works")
        series_query = urllib.parse.urlencode(
            {
                "seriesView": "true",
                "contentType": "comic",
                "libraryIds": library_id,
                "search": detail.get("title", ""),
                "page": 1,
                "pageSize": 50,
            }
        )
        series_status, series_payload = self.client.json(
            f"/api/comics?{series_query}"
        )
        series_works = payload_items(series_payload, "comics", "works")
        opds = self._comparison_opds_snapshot(detail, library_id)
        reading = self._comparison_reading_snapshot(
            detail,
            units,
            library_id,
            variant_report,
        )
        return {
            "work": {
                key: detail.get(key)
                for key in (
                    "id",
                    "libraryId",
                    "title",
                    "rootPath",
                    "seriesId",
                    "metadataHostType",
                    "metadataHostId",
                    "representativeComicId",
                    "coverComicId",
                    "coverUrl",
                    "coverAspectRatio",
                    "itemCount",
                    "pageCount",
                    "fileSize",
                    "addedAt",
                    "updatedAt",
                    "author",
                    "publisher",
                    "year",
                    "description",
                    "language",
                    "genre",
                )
            },
            "tags": sorted(
                str(item.get("name", ""))
                for item in payload_items(detail, "tags")
            ),
            "categories": sorted(
                str(item.get("slug", ""))
                for item in payload_items(detail, "categories")
            ),
            "expectedBoundMetadata": {
                "tag": tag_name,
                "categoryName": category_name,
                "categorySlug": category_slug,
            },
            "cover": {
                "status": cover_status,
                "sourceIsUnit": detail.get("coverComicId")
                in {unit.get("comicId") for unit in units},
                **cover,
            },
            "units": unit_snapshots,
            "physicalAcquisitionMimes": sorted(
                {
                    mime
                    for mime in (
                        acquisition_mime_for_path(
                            str(unit.get("relativePath", ""))
                        )
                        for unit in units
                    )
                    if mime
                }
            ),
            "flattenedPages": flattened_pages,
            "detailPage": {
                "status": frontend_status,
                "isHtml": b"<html" in frontend_payload.lower(),
            },
            "search": {
                "status": search_status,
                "workIds": [item.get("id") for item in search_works],
            },
            "seriesView": {
                "status": series_status,
                "workIds": [item.get("id") for item in series_works],
            },
            "opds": opds,
            "reading": reading,
        }

    def _validate_comparison_variant(
        self,
        variant: dict,
        snapshot: dict,
        variant_report: dict,
    ) -> None:
        comparison = self.manifest.get("comparison", {})
        work = snapshot.get("work", {})
        units = snapshot.get("units", [])
        strict_checks = (
            ("title", comparison.get("workTitle"), work.get("title")),
            ("author", comparison.get("author"), work.get("author")),
            (
                "description",
                comparison.get("description"),
                work.get("description"),
            ),
            ("language", comparison.get("language"), work.get("language")),
            ("total page count", comparison.get("totalPages"), work.get("pageCount")),
            (
                "Unit count",
                variant.get("expectedUnits"),
                len(units),
            ),
            (
                "Unit titles",
                variant.get("expectedChapterTitles"),
                [unit.get("displayLabel") for unit in units],
            ),
            (
                "Unit page counts",
                variant.get("expectedUnitPageCounts"),
                [unit.get("pageCount") for unit in units],
            ),
        )
        for label, expected, actual in strict_checks:
            if expected != actual:
                self._comparison_violation(
                    variant_report,
                    f"{label} differs",
                    expected,
                    actual,
                )
        expected_genres = set(comparison.get("genreTerms", []))
        actual_genres = {
            value.strip()
            for value in re.split(r"[,，;/]", str(work.get("genre", "")))
            if value.strip()
        }
        if not expected_genres.issubset(actual_genres):
            self._comparison_violation(
                variant_report,
                "genre metadata differs",
                sorted(expected_genres),
                sorted(actual_genres),
            )
        expected_tag = snapshot.get("expectedBoundMetadata", {}).get("tag")
        expected_category = snapshot.get("expectedBoundMetadata", {}).get(
            "categorySlug"
        )
        if expected_tag not in snapshot.get("tags", []):
            self._comparison_violation(
                variant_report,
                "bound tag is missing from Work DTO",
                expected_tag,
                snapshot.get("tags"),
            )
        if expected_category not in snapshot.get("categories", []):
            self._comparison_violation(
                variant_report,
                "bound category is missing from Work DTO",
                expected_category,
                snapshot.get("categories"),
            )

        cover = snapshot.get("cover", {})
        expected_ratio = float(comparison.get("coverAspectRatio", 0))
        if cover.get("status") != 200 or not cover.get("sourceIsUnit"):
            self._comparison_violation(
                variant_report,
                "Work cover source is not readable or is not backed by a Unit",
                True,
                cover,
            )
        if abs(float(work.get("coverAspectRatio", 0) or 0) - expected_ratio) > 0.02:
            self._comparison_violation(
                variant_report,
                "Work DTO cover aspect ratio differs",
                expected_ratio,
                work.get("coverAspectRatio"),
            )
        if abs(float(cover.get("aspectRatio", 0) or 0) - expected_ratio) > 0.02:
            self._comparison_violation(
                variant_report,
                "rendered cover aspect ratio differs",
                expected_ratio,
                cover.get("aspectRatio"),
            )
        cover_distance = hash_distance(
            str(cover.get("averageHash", "0")),
            str(comparison.get("coverAverageHash", "0")),
        )
        cover_limit = 8 if variant.get("code") == "G" else 0
        if cover_distance > cover_limit:
            self._comparison_violation(
                variant_report,
                "rendered cover content differs",
                f"average-hash distance <= {cover_limit}",
                cover_distance,
            )

        actual_average_hashes = [
            page.get("averageHash")
            for page in snapshot.get("flattenedPages", [])
        ]
        expected_average_hashes = list(
            comparison.get("canonicalPageAverageHash", {}).values()
        )
        if len(actual_average_hashes) != len(expected_average_hashes):
            self._comparison_violation(
                variant_report,
                "flattened readable page count differs",
                len(expected_average_hashes),
                len(actual_average_hashes),
            )
        else:
            limit = 8 if variant.get("code") == "G" else 0
            distances = [
                hash_distance(str(actual), str(expected))
                for actual, expected in zip(
                    actual_average_hashes,
                    expected_average_hashes,
                )
            ]
            if any(distance > limit for distance in distances):
                self._comparison_violation(
                    variant_report,
                    "page content/order differs",
                    f"all average-hash distances <= {limit}",
                    distances,
                )
        if variant.get("code") != "G":
            actual_sha = [
                page.get("sha256")
                for page in snapshot.get("flattenedPages", [])
            ]
            expected_sha = list(
                comparison.get("canonicalPageSha256", {}).values()
            )
            if actual_sha != expected_sha:
                self._comparison_violation(
                    variant_report,
                    "non-PDF page bytes differ from canonical content/order",
                    expected_sha,
                    actual_sha,
                )

        normalized_starts = [
            unit.get("normalizedStartPage") for unit in units
        ]
        normalized_ends = [unit.get("normalizedEndPage") for unit in units]
        expected_starts = []
        cursor = 0
        for count in variant.get("expectedUnitPageCounts", []):
            expected_starts.append(cursor)
            cursor += int(count)
        expected_ends = [
            start + int(count) - 1
            for start, count in zip(
                expected_starts,
                variant.get("expectedUnitPageCounts", []),
            )
        ]
        if normalized_starts != expected_starts or normalized_ends != expected_ends:
            self._comparison_violation(
                variant_report,
                "normalized Unit start/end pages differ",
                {"starts": expected_starts, "ends": expected_ends},
                {"starts": normalized_starts, "ends": normalized_ends},
            )
        invalid_physical_windows = [
            {
                "start": unit.get("startPage"),
                "end": unit.get("endPage"),
                "pageCount": unit.get("pageCount"),
                "pageListTotal": unit.get("pageListTotal"),
            }
            for unit in units
            if int(unit.get("endPage", -1)) - int(unit.get("startPage", 0)) + 1
            != int(unit.get("pageCount", 0))
            or int(unit.get("endPage", -1))
            >= int(unit.get("pageListTotal", 0))
        ]
        if invalid_physical_windows:
            self._comparison_violation(
                variant_report,
                "physical Unit start/end page windows are invalid",
                [],
                invalid_physical_windows,
            )

        if snapshot.get("detailPage") != {"status": 200, "isHtml": True}:
            self._comparison_violation(
                variant_report,
                "Work detail page capability differs",
                {"status": 200, "isHtml": True},
                snapshot.get("detailPage"),
            )
        work_id = work.get("id")
        for name in ("search", "seriesView"):
            value = snapshot.get(name, {})
            if value.get("status") != 200 or value.get("workIds") != [work_id]:
                self._comparison_violation(
                    variant_report,
                    f"{name} does not return exactly one Work",
                    [work_id],
                    value,
                )

        opds = snapshot.get("opds", {})
        if (
            opds.get("searchStatus") != 200
            or not opds.get("searchValid")
            or opds.get("searchTitles") != [comparison.get("workTitle")]
        ):
            self._comparison_violation(
                variant_report,
                "OPDS search structure differs",
                [comparison.get("workTitle")],
                opds,
            )
        expected_tag = snapshot.get("expectedBoundMetadata", {}).get("tag")
        expected_category_name = snapshot.get(
            "expectedBoundMetadata",
            {},
        ).get("categoryName")
        expected_opds_metadata = {
            value
            for value in (expected_tag, expected_category_name)
            if value
        }
        if not expected_opds_metadata.issubset(
            set(opds.get("searchCategories", []))
        ):
            self._comparison_violation(
                variant_report,
                "OPDS Work metadata differs from tags/categories in Work DTO",
                sorted(expected_opds_metadata),
                opds.get("searchCategories"),
            )
        if opds.get("unitTitles") != variant.get("expectedChapterTitles"):
            self._comparison_violation(
                variant_report,
                "OPDS Unit titles/order differ",
                variant.get("expectedChapterTitles"),
                opds.get("unitTitles"),
            )
        if opds.get("unitPageCounts") != variant.get("expectedUnitPageCounts"):
            self._comparison_violation(
                variant_report,
                "OPDS Unit page counts differ",
                variant.get("expectedUnitPageCounts"),
                opds.get("unitPageCounts"),
            )
        if opds.get("acquisitionMimes") != variant.get(
            "allowedAcquisitionMimes",
            [],
        ):
            self._comparison_violation(
                variant_report,
                "OPDS physical acquisition MIME differs from allowed format difference",
                variant.get("allowedAcquisitionMimes", []),
                opds.get("acquisitionMimes"),
            )
        expected_physical_mime = variant.get("physicalAcquisitionMime")
        if expected_physical_mime:
            expected_physical_mimes = [expected_physical_mime]
        else:
            expected_physical_mimes = variant.get(
                "allowedAcquisitionMimes",
                [],
            )
        if snapshot.get("physicalAcquisitionMimes") != expected_physical_mimes:
            self._comparison_violation(
                variant_report,
                "physical acquisition MIME differs from allowed format difference",
                expected_physical_mimes,
                snapshot.get("physicalAcquisitionMimes"),
            )
        if opds.get("pseLinkCount") != variant.get("expectedUnits"):
            self._comparison_violation(
                variant_report,
                "OPDS PSE Unit count differs",
                variant.get("expectedUnits"),
                opds.get("pseLinkCount"),
            )

        reading = snapshot.get("reading", {})
        if not reading.get("skipped"):
            expected_continue_index = (
                0
                if variant.get("code") == "G"
                else int(variant.get("expectedUnits", 1)) - 1
            )
            reading_checks = (
                (
                    "reading start/end pages",
                    [0, int(comparison.get("totalPages", 0)) - 1],
                    reading.get("checkpointGlobalPages"),
                ),
                (
                    "continue Unit index",
                    expected_continue_index,
                    reading.get("continueUnitIndex"),
                ),
                (
                    "continue global page",
                    int(comparison.get("totalPages", 0)) - 1,
                    reading.get("continueGlobalPage"),
                ),
                ("history Work count", 1, reading.get("historyWorkCount")),
                ("reading session delta", 2, reading.get("sessionDelta")),
                ("statistics Work delta", 1, reading.get("workCountDelta")),
            )
            for label, expected, actual in reading_checks:
                if expected != actual:
                    self._comparison_violation(
                        variant_report,
                        f"{label} differs",
                        expected,
                        actual,
                    )

    def _normalized_parallel_semantics(
        self,
        snapshot: dict,
        include_unit_structure: bool,
    ) -> dict:
        work = snapshot.get("work", {})
        opds = snapshot.get("opds", {})
        reading = snapshot.get("reading", {})
        value = {
            "title": work.get("title"),
            "author": work.get("author"),
            "description": work.get("description"),
            "language": work.get("language"),
            "genre": work.get("genre"),
            "totalPages": work.get("pageCount"),
            "coverAspectRatio": round(
                float(work.get("coverAspectRatio", 0) or 0),
                4,
            ),
            "coverAvailable": snapshot.get("cover", {}).get("status") == 200,
            "coverSourceIsUnit": snapshot.get("cover", {}).get(
                "sourceIsUnit"
            ),
            "tags": snapshot.get("tags", []),
            "categories": snapshot.get("categories", []),
            "detailPage": snapshot.get("detailPage"),
            "searchWorkCount": len(snapshot.get("search", {}).get("workIds", [])),
            "seriesViewWorkCount": len(
                snapshot.get("seriesView", {}).get("workIds", [])
            ),
            "opdsSearchTitles": opds.get("searchTitles"),
            "opdsSearchCategories": opds.get("searchCategories"),
            "continueGlobalPage": reading.get("continueGlobalPage"),
            "historyWorkCount": reading.get("historyWorkCount"),
            "statisticsWorkDelta": reading.get("workCountDelta"),
        }
        if include_unit_structure:
            value.update(
                {
                    "unitTitles": [
                        unit.get("displayLabel")
                        for unit in snapshot.get("units", [])
                    ],
                    "unitPageCounts": [
                        unit.get("pageCount")
                        for unit in snapshot.get("units", [])
                    ],
                    "normalizedStarts": [
                        unit.get("normalizedStartPage")
                        for unit in snapshot.get("units", [])
                    ],
                    "normalizedEnds": [
                        unit.get("normalizedEndPage")
                        for unit in snapshot.get("units", [])
                    ],
                    "opdsUnitTitles": opds.get("unitTitles"),
                    "opdsUnitPageCounts": opds.get("unitPageCounts"),
                    "coverAverageHash": snapshot.get("cover", {}).get(
                        "averageHash"
                    ),
                    "coverSha256": snapshot.get("cover", {}).get("sha256"),
                    "pageAverageHashes": [
                        page.get("averageHash")
                        for page in snapshot.get("flattenedPages", [])
                    ],
                    "pageSha256": [
                        page.get("sha256")
                        for page in snapshot.get("flattenedPages", [])
                    ],
                }
            )
        return value

    def verify_parallel_library_comparison(self) -> None:
        variants = self.manifest.get("libraries", [])
        if not variants:
            return
        self.comparison_report["canonical"] = self.manifest.get(
            "comparison",
            {},
        )
        status, libraries_payload = self.client.json("/api/admin/libraries")
        libraries = payload_items(libraries_payload, "libraries")
        root_to_library = {}
        for library in libraries:
            roots = library.get("rootPaths") or [library.get("rootPath")]
            for root in roots:
                if root:
                    root_to_library[str(root)] = library
        self.check(
            status == 200,
            "parallel comparison can list fixture libraries",
        )

        unique = uuid.uuid4().hex[:12]
        tag_name = f"parallel-tag-{unique}"
        category_name = f"Parallel Category {unique}"
        category_slug = f"parallel-category-{unique}"
        create_status, _ = self.client.json(
            "/api/categories/create",
            "POST",
            {
                "name": category_name,
                "slug": category_slug,
                "icon": "book",
            },
        )
        self.check(
            create_status == 200,
            "parallel comparison category is created",
        )

        try:
            for variant in variants:
                code = str(variant.get("code", ""))
                variant_report = {
                    "code": code,
                    "libraryName": variant.get("libraryName"),
                    "format": variant.get("format"),
                    "allowedSemanticDifferences": variant.get(
                        "allowedSemanticDifferences",
                        {},
                    ),
                    "allowedPhysicalDifferences": {
                        "acquisitionMimes": variant.get(
                            "allowedAcquisitionMimes",
                            [],
                        ),
                        "physicalAcquisitionMime": variant.get(
                            "physicalAcquisitionMime",
                            (
                                variant.get("allowedAcquisitionMimes") or [None]
                            )[0],
                        ),
                    },
                    "violations": [],
                }
                self.comparison_report["variants"].append(variant_report)
                library = root_to_library.get(str(variant.get("rootPath", "")))
                if not library:
                    self._comparison_violation(
                        variant_report,
                        "expected fixture library is missing",
                        variant.get("rootPath"),
                        sorted(root_to_library),
                    )
                    continue
                library_id = str(library.get("id", ""))
                works = self._wait_for_library_work(library_id)
                if len(works) != 1:
                    self._comparison_violation(
                        variant_report,
                        "library does not contain exactly one Work",
                        1,
                        len(works),
                    )
                    continue
                work = works[0]
                if work.get("title") != self.manifest.get(
                    "comparison",
                    {},
                ).get("workTitle"):
                    self._comparison_violation(
                        variant_report,
                        "library Work title differs before metadata binding",
                        self.manifest.get("comparison", {}).get("workTitle"),
                        work.get("title"),
                    )

                host_type = str(work.get("metadataHostType", "comic"))
                host_id = str(
                    work.get("metadataHostId")
                    or work.get("representativeComicId")
                    or ""
                )
                target_type = "series" if host_type == "series" else "comics"
                target = (
                    f"/api/{target_type}/"
                    f"{urllib.parse.quote(host_id, safe='')}"
                )
                tag_status, _ = self.client.json(
                    target + "/tags",
                    "PUT",
                    {"tags": [tag_name]},
                )
                category_status, _ = self.client.json(
                    target + "/categories",
                    "PUT",
                    {"categorySlugs": [category_slug]},
                )
                if tag_status != 200:
                    self._comparison_violation(
                        variant_report,
                        "metadata-host tag binding failed",
                        200,
                        tag_status,
                    )
                if category_status != 200:
                    self._comparison_violation(
                        variant_report,
                        "metadata-host category binding failed",
                        200,
                        category_status,
                    )
                time.sleep(0.2)
                refreshed = self._wait_for_library_work(library_id, retries=5)
                if len(refreshed) != 1:
                    continue
                snapshot = self._comparison_variant_snapshot(
                    variant,
                    library_id,
                    refreshed[0],
                    tag_name,
                    category_name,
                    category_slug,
                    variant_report,
                )
                variant_report["libraryId"] = library_id
                variant_report["snapshot"] = snapshot
                self._validate_comparison_variant(
                    variant,
                    snapshot,
                    variant_report,
                )
                if not variant_report["violations"]:
                    self.check(
                        True,
                        f"{code}: unified Work comparison matches canonical semantics",
                    )
            reports_by_code = {
                report.get("code"): report
                for report in self.comparison_report["variants"]
                if report.get("snapshot")
            }
            baseline_report = reports_by_code.get("A")
            comparisons = []
            if baseline_report:
                baseline = self._normalized_parallel_semantics(
                    baseline_report["snapshot"],
                    include_unit_structure=True,
                )
                for code in "BCDEF":
                    report = reports_by_code.get(code)
                    if not report:
                        continue
                    actual = self._normalized_parallel_semantics(
                        report["snapshot"],
                        include_unit_structure=True,
                    )
                    equal = actual == baseline
                    comparisons.append(
                        {
                            "baseline": "A",
                            "variant": code,
                            "equal": equal,
                            "allowedDifferences": report.get(
                                "allowedPhysicalDifferences",
                                {},
                            ),
                            "baselineSemantics": baseline,
                            "actualSemantics": actual,
                        }
                    )
                    if not equal:
                        self._comparison_violation(
                            report,
                            "normalized Work/display semantics differ from variant A",
                            baseline,
                            actual,
                        )
                pdf_report = reports_by_code.get("G")
                if pdf_report:
                    baseline_common = self._normalized_parallel_semantics(
                        baseline_report["snapshot"],
                        include_unit_structure=False,
                    )
                    pdf_common = self._normalized_parallel_semantics(
                        pdf_report["snapshot"],
                        include_unit_structure=False,
                    )
                    equal = pdf_common == baseline_common
                    comparisons.append(
                        {
                            "baseline": "A",
                            "variant": "G",
                            "equal": equal,
                            "allowedDifferences": pdf_report.get(
                                "allowedSemanticDifferences",
                                {},
                            ),
                            "baselineSemantics": baseline_common,
                            "actualSemantics": pdf_common,
                        }
                    )
                    if not equal:
                        self._comparison_violation(
                            pdf_report,
                            "PDF common Work/display semantics differ from variant A",
                            baseline_common,
                            pdf_common,
                        )
            self.comparison_report["crossVariantComparisons"] = comparisons
        finally:
            self.client.json("/api/tags", "DELETE", {"name": tag_name})
            self.client.json(
                f"/api/categories/{urllib.parse.quote(category_slug, safe='')}",
                "DELETE",
                {},
            )
        self.comparison_report["status"] = (
            "pass"
            if not self.comparison_report["violations"]
            else "fail"
        )

    def write_comparison_report(self) -> None:
        if self.comparison_report_path is None:
            return
        if self.comparison_report.get("status") == "not-run":
            self.comparison_report["status"] = (
                "fail" if self.failures else "not-applicable"
            )
        self.comparison_report["passes"] = self.passes
        self.comparison_report["failures"] = len(self.failures)
        self.comparison_report_path.parent.mkdir(parents=True, exist_ok=True)
        self.comparison_report_path.write_text(
            json.dumps(
                self.comparison_report,
                ensure_ascii=False,
                indent=2,
            ),
            encoding="utf-8",
        )
        print(f"comparison report: {self.comparison_report_path.resolve()}")

    def verify_opds(self, favorite_work_id: str | None = None) -> None:
        feeds = {
            "all": "/api/opds/all?page=1&pageSize=500",
            "works": "/api/opds/works?page=1&pageSize=500",
            "series": "/api/opds/series?page=1&pageSize=500",
            "recent": "/api/opds/recent?page=1&pageSize=500",
        }
        feed_entries: dict[str, list[ET.Element]] = {}
        for name, path in feeds.items():
            status, headers, payload = self.client.request(path)
            root, entries = parse_feed(payload)
            self.check(
                status == 200
                and root is not None
                and "xml" in str(headers.get("Content-Type", "")).lower(),
                f"OPDS {name} feed is valid XML",
            )
            feed_entries[name] = entries
        expected_count = len(self.works)
        for name in ("all", "works", "series"):
            self.check(
                len(feed_entries.get(name, [])) == expected_count,
                f"OPDS {name} contains one entry per Work",
            )
        self.check(
            len(feed_entries.get("recent", [])) == expected_count,
            "OPDS recent is aggregated at Work level",
        )

        if self.works:
            title = str(self.works[0].get("title", ""))
            query = urllib.parse.urlencode(
                {
                    "q": title,
                    "libraryIds": self.works[0].get("libraryId", ""),
                    "page": 1,
                    "pageSize": 50,
                }
            )
            status, _, payload = self.client.request(f"/api/opds/search?{query}")
            root, entries = parse_feed(payload)
            self.check(status == 200 and root is not None, "OPDS search is valid XML")
            self.check(
                len(entries) == 1 and feed_titles(entries)[0] == title,
                "OPDS search returns one Work instead of its Units",
            )

        if favorite_work_id:
            status, _, payload = self.client.request(
                "/api/opds/favorites?page=1&pageSize=500"
            )
            root, entries = parse_feed(payload)
            self.check(
                status == 200 and root is not None,
                "OPDS favorites feed is valid XML",
            )
            favorite_detail = self.details.get(favorite_work_id, {})
            self.check(
                str(favorite_detail.get("title", "")) in feed_titles(entries),
                "OPDS favorites aggregates the favorited Work",
            )

        for work in self.works:
            work_id = urllib.parse.quote(str(work.get("id", "")), safe="")
            status, _, payload = self.client.request(
                opds_work_detail_path(work_id)
            )
            root, entries = parse_feed(payload)
            units = payload_items(self.details.get(str(work.get("id")), {}), "units")
            self.check(
                status == 200 and root is not None and len(entries) == len(units),
                f"OPDS Work detail exposes one entry per Unit: "
                f"{work.get('title')}",
            )
            acquisition_hrefs: list[str] = []
            for entry in entries:
                for link in entry.findall("atom:link", ATOM):
                    rel = link.attrib.get("rel", "")
                    if "acquisition" in rel:
                        acquisition_hrefs.append(link.attrib.get("href", ""))
            self.check(
                len(acquisition_hrefs) == len(set(acquisition_hrefs)),
                f"OPDS Work detail has no duplicate acquisition links: "
                f"{work.get('title')}",
            )

    def verify_removed_collections(self) -> None:
        for path in (
            "/api/groups",
            "/api/groups/comic-map",
            "/api/collections",
        ):
            status, _, _ = self.client.request(path)
            self.check(
                status == 404,
                f"removed collection API is absent: {path} (HTTP {status})",
            )

        status, _, html = self.client.request("/books")
        self.check(status == 200 and b"<html" in html.lower(), "/books UI loads")
        sources = re.findall(
            rb"""<script[^>]+src=["']([^"']+)["']""",
            html,
            flags=re.IGNORECASE,
        )
        pending = [
            source.decode("utf-8", errors="ignore")
            for source in sources
        ]
        visited: set[str] = set()
        javascript = bytearray()
        while pending and len(visited) < 300 and len(javascript) < 100_000_000:
            candidate = pending.pop()
            absolute = urllib.parse.urljoin(self.base_url + "/books", candidate)
            parsed = urllib.parse.urlsplit(absolute)
            if (
                parsed.hostname != "192.168.2.9"
                or parsed.port != 17680
                or not parsed.path.endswith(".js")
            ):
                continue
            path = parsed.path
            if path in visited:
                continue
            visited.add(path)
            asset_status, _, payload = self.client.request(path)
            if asset_status != 200:
                continue
            javascript.extend(payload)
            for reference in re.findall(
                rb"""["']([^"']+\.js(?:\?[^"']*)?)["']""",
                payload,
            ):
                pending.append(
                    urllib.parse.urljoin(
                        path,
                        reference.decode("utf-8", errors="ignore"),
                    )
                )
        decoded = javascript.decode("utf-8", errors="ignore")
        self.check(bool(visited), "frontend JavaScript assets are discoverable")
        self.check(
            '"/collections"' not in decoded and "'/collections'" not in decoded,
            "frontend bundle has no collections route",
        )
        self.check(
            '"/group/' not in decoded and "'/group/" not in decoded,
            "frontend bundle has no collection detail route",
        )

    def exercise_two_unit_reading(self) -> str | None:
        if not self.exercise_state or not self.works:
            return None
        target = next(
            (
                work
                for work in self.works
                if len(
                    {
                        str(unit.get("comicId", ""))
                        for unit in payload_items(
                            self.details.get(str(work.get("id")), {}),
                            "units",
                        )
                        if unit.get("comicId")
                    }
                ) >= 2
            ),
            None,
        )
        self.check(
            target is not None,
            "fixture contains a multi-Unit Work backed by distinct physical Comics",
        )
        if target is None:
            return None
        work_id = str(target.get("id"))
        detail = self.details.get(work_id, {})
        units = payload_items(detail, "units")
        selected_units: list[dict] = []
        selected_comics: set[str] = set()
        for unit in units:
            comic_id = str(unit.get("comicId", ""))
            if not comic_id or comic_id in selected_comics:
                continue
            selected_units.append(unit)
            selected_comics.add(comic_id)
            if len(selected_units) == 2:
                break
        self.check(
            len(selected_units) == 2,
            "two distinct Units are selected for continuous reading",
        )
        if len(selected_units) != 2:
            return work_id

        status, baseline_stats = self.client.json(work_stats_path())
        baseline_total_sessions = (
            int(baseline_stats.get("totalSessions", 0))
            if status == 200 and isinstance(baseline_stats, dict)
            else 0
        )
        baseline_total_works = (
            int(baseline_stats.get("totalComicsRead", 0))
            if status == 200 and isinstance(baseline_stats, dict)
            else 0
        )
        self.check(status == 200, "baseline Work statistics are readable")
        baseline_sessions = payload_items(baseline_stats, "recentSessions")
        baseline_history = payload_items(baseline_stats, "history")
        baseline_history_ids = [
            str(item.get("id", "")) for item in baseline_history if item.get("id")
        ]
        baseline_work_was_read = work_id in set(baseline_history_ids)
        self.check(
            len(baseline_history_ids) == len(set(baseline_history_ids)),
            "baseline reading history is already aggregated by Work",
        )

        read_results: list[tuple[dict, int, int]] = []
        session_ids: list[int] = []
        for index, unit in enumerate(selected_units):
            raw_comic_id = str(unit.get("comicId", ""))
            comic_id = urllib.parse.quote(raw_comic_id, safe="")
            unit_page_count = int(unit.get("pageCount", 0) or 0)
            relative_page = 1 if unit_page_count > 1 else 0
            absolute_page = int(unit.get("startPage", 0) or 0) + relative_page
            status, pages = self.client.json(f"/api/comics/{comic_id}/pages")
            total_pages = (
                int(pages.get("totalPages", 0))
                if isinstance(pages, dict)
                else 0
            )
            self.check(
                status == 200 and total_pages > absolute_page,
                f"continuous reading Unit {index + 1} Page exists",
            )
            status, _ = self.client.json(
                f"/api/comics/{comic_id}/progress",
                "PUT",
                {"page": absolute_page, "totalPages": total_pages},
            )
            self.check(
                status == 200,
                f"continuous reading Unit {index + 1} progress update succeeds",
            )
            session_status, session = self.client.json(
                "/api/stats/session",
                "POST",
                {"comicId": raw_comic_id, "startPage": absolute_page},
            )
            session_id = (
                int(session.get("sessionId"))
                if isinstance(session, dict) and session.get("sessionId")
                else 0
            )
            self.check(
                session_status == 200 and session_id > 0,
                f"continuous reading Unit {index + 1} history session starts",
            )
            if session_id > 0:
                status, _ = self.client.json(
                    "/api/stats/session",
                    "PUT",
                    {
                        "sessionId": session_id,
                        "endPage": absolute_page,
                        "duration": 1,
                    },
                )
                self.check(
                    status == 200,
                    f"continuous reading Unit {index + 1} history session ends",
                )
                session_ids.append(session_id)
            read_results.append((unit, relative_page, absolute_page))
            if index == 0:
                time.sleep(1.05)

        last_unit, last_relative_page, last_absolute_page = read_results[-1]
        status, refreshed = self.client.json(
            f"/api/works/{urllib.parse.quote(work_id, safe='')}"
        )
        self.details[work_id] = (
            refreshed if isinstance(refreshed, dict) else detail
        )
        self.check(
            status == 200,
            "Work detail refreshes after reading two Units",
        )
        if isinstance(refreshed, dict):
            self.check(
                refreshed.get("continueUnitId") == last_unit.get("id"),
                "continue reading points to the second and latest Unit",
            )
            self.check(
                int(refreshed.get("continuePage", -1)) == last_absolute_page,
                "continue reading preserves the latest Unit physical Page",
            )
            refreshed_units = payload_items(refreshed, "units")
            refreshed_unit = next(
                (
                    unit
                    for unit in refreshed_units
                    if unit.get("id") == last_unit.get("id")
                ),
                {},
            )
            self.check(
                int(refreshed_unit.get("lastReadPage", -1))
                == last_relative_page,
                "continue reading preserves the latest Unit relative Page",
            )

        status, sorted_payload = self.client.json(
            "/api/works?sortBy=lastReadAt&sortOrder=desc&page=1&pageSize=20"
        )
        sorted_works = payload_items(sorted_payload, "works")
        self.check(
            status == 200
            and sorted_works
            and sorted_works[0].get("id") == work_id,
            "two read Units produce one latest Work in Work history ordering",
        )
        self.check(
            sum(1 for work in sorted_works if work.get("id") == work_id) == 1,
            "Work history contains exactly one card for both read Units",
        )
        status, stats = self.client.json(work_stats_path())
        sessions = payload_items(stats, "recentSessions")
        read_comic_ids = {
            str(unit.get("comicId", "")) for unit in selected_units
        }
        matching_sessions = [
            session
            for session in sessions
            if str(session.get("comicId", "")) in read_comic_ids
            or str(session.get("workId", "")) == work_id
            or int(session.get("id", 0) or 0) in session_ids
        ]
        projected_history_work_ids = {
            str(session.get("workId") or work_id)
            for session in matching_sessions
        }
        self.check(
            status == 200
            and int(stats.get("totalSessions", 0) if isinstance(stats, dict) else 0)
            == baseline_total_sessions + 2,
            "statistics records both Unit reading sessions",
        )
        self.check(
            len(matching_sessions) >= 1
            and projected_history_work_ids == {work_id},
            "reading history projects both Units to one Work",
        )
        self.check(
            status == 200
            and int(stats.get("totalComicsRead", -1) if isinstance(stats, dict) else -1)
            == expected_work_count_after_read(
                baseline_total_works,
                baseline_work_was_read,
            ),
            "statistics counts the two Units as one Work rather than two books",
        )
        return work_id

    def favorite_for_opds(self) -> tuple[str | None, str | None]:
        if not self.exercise_state or not self.works:
            return None, None
        work = self.works[0]
        detail = self.details.get(str(work.get("id")), {})
        comic_id = str(
            detail.get("representativeComicId")
            or detail.get("coverComicId")
            or ""
        )
        if not comic_id:
            return None, None
        was_favorite = bool(detail.get("isFavorite"))
        if not was_favorite:
            status, response = self.client.json(
                f"/api/comics/{urllib.parse.quote(comic_id, safe='')}/favorite",
                "PUT",
                {},
            )
            self.check(
                status == 200
                and isinstance(response, dict)
                and response.get("isFavorite") is True,
                "fixture Work can be favorited through its metadata host",
            )
            time.sleep(0.2)
        return str(work.get("id")), None if was_favorite else comic_id

    def cleanup_favorite(self, comic_id: str | None) -> None:
        if comic_id:
            self.client.json(
                f"/api/comics/{urllib.parse.quote(comic_id, safe='')}/favorite",
                "PUT",
                {},
            )

    def run(self) -> int:
        metadata_state: tuple[str, str] | None = None
        cleanup_comic: str | None = None
        try:
            self.verify_auth_and_permissions()
            self.load_works()
            self.verify_work_unit_page_model()
            self.verify_series_view_and_filters()
            if self.manifest.get("libraries"):
                self.verify_parallel_library_comparison()
            else:
                metadata_state = self.verify_work_metadata_filters()
                self.exercise_two_unit_reading()
            favorite_work_id, cleanup_comic = self.favorite_for_opds()
            self.verify_opds(favorite_work_id)
            self.verify_removed_collections()
        finally:
            self.cleanup_favorite(cleanup_comic)
            self.cleanup_work_metadata(metadata_state)
            self.write_comparison_report()
        if self.failures:
            print(f"\n{len(self.failures)} checks failed; {self.passes} passed.")
            for failure in self.failures:
                print(f" - {failure}")
            return 1
        total_units = sum(
            len(payload_items(detail, "units"))
            for detail in self.details.values()
        )
        print(
            f"\nAll {self.passes} checks passed: "
            f"{len(self.works)} Works, {total_units} Units."
        )
        return 0


def load_manifest(path: Path | None) -> dict:
    if path is None:
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=SAFE_DEFAULT_BASE_URL)
    parser.add_argument("--username", default="admin")
    parser.add_argument("--manifest", type=Path)
    parser.add_argument(
        "--comparison-report",
        type=Path,
        default=Path(".tmp/unified_work_comparison_report.json"),
    )
    parser.add_argument(
        "--read-only",
        action="store_true",
        help="skip favorite/progress/history state checks",
    )
    args = parser.parse_args()
    try:
        suite = VerificationSuite(
            args.base_url,
            args.username,
            resolve_admin_password(),
            manifest=load_manifest(args.manifest),
            exercise_state=not args.read_only,
            comparison_report_path=args.comparison_report,
        )
    except (
        ValueError,
        RuntimeError,
        OSError,
        json.JSONDecodeError,
    ) as error:
        print(f"configuration error: {error}", file=sys.stderr)
        return 2
    return suite.run()


if __name__ == "__main__":
    sys.exit(main())
