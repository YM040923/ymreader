#!/usr/bin/env python3
"""Generate parallel-format fixtures for unified Work comparison."""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import shutil
from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile

from PIL import Image, ImageDraw


DEFAULT_ROOT = Path(__file__).resolve().parents[1] / ".tmp" / "unified_work_fixture"
WORK_TITLE = "统一格式对照漫画"
WORK_AUTHOR = "YMReader 验收作者"
WORK_DESCRIPTION = "同一作品以不同物理整理格式存储，用于验证统一 Work 模型。"
WORK_LANGUAGE = "zh-CN"
WORK_GENRE = "平行验收,统一模型"
CHAPTERS = (
    ("第001话 起点", "1"),
    ("第002话 回声", "2"),
)

PARALLEL_VARIANTS = (
    {
        "code": "A",
        "directory": "A-folder-chapters",
        "libraryName": "A 章节图片文件夹",
        "format": "work-directory/chapter-image-folders",
        "expectedUnits": 2,
        "expectedPhysicalComics": 2,
        "expectedUnitPageCounts": [3, 3],
        "expectedInternalUnits": False,
        "allowedAcquisitionMimes": [],
    },
    {
        "code": "B",
        "directory": "B-multi-cbz",
        "libraryName": "B 作品目录多 CBZ",
        "format": "work-directory/multiple-cbz",
        "expectedUnits": 2,
        "expectedPhysicalComics": 2,
        "expectedUnitPageCounts": [3, 3],
        "expectedInternalUnits": False,
        "allowedAcquisitionMimes": ["application/vnd.comicbook+zip"],
    },
    {
        "code": "C",
        "directory": "C-multi-zip",
        "libraryName": "C 作品目录多 ZIP",
        "format": "work-directory/multiple-zip",
        "expectedUnits": 2,
        "expectedPhysicalComics": 2,
        "expectedUnitPageCounts": [3, 3],
        "expectedInternalUnits": False,
        "allowedAcquisitionMimes": ["application/vnd.comicbook+zip"],
    },
    {
        "code": "D",
        "directory": "D-single-large-zip",
        "libraryName": "D 单大 ZIP 内章节",
        "format": "single-large-zip/internal-chapters",
        "expectedUnits": 2,
        "expectedPhysicalComics": 1,
        "expectedUnitPageCounts": [3, 3],
        "expectedInternalUnits": True,
        "allowedAcquisitionMimes": [],
        "physicalAcquisitionMime": "application/vnd.comicbook+zip",
    },
    {
        "code": "E",
        "directory": "E-single-large-cbz-wrapper",
        "libraryName": "E 单大 CBZ 单一作品根目录",
        "format": "single-large-cbz/work-wrapper/internal-chapters",
        "expectedUnits": 2,
        "expectedPhysicalComics": 1,
        "expectedUnitPageCounts": [3, 3],
        "expectedInternalUnits": True,
        "allowedAcquisitionMimes": [],
        "physicalAcquisitionMime": "application/vnd.comicbook+zip",
    },
    {
        "code": "F",
        "directory": "F-mixed-folder-archive",
        "libraryName": "F 混合文件夹与压缩包",
        "format": "work-directory/mixed-folder-cbz",
        "expectedUnits": 2,
        "expectedPhysicalComics": 2,
        "expectedUnitPageCounts": [3, 3],
        "expectedInternalUnits": False,
        "allowedAcquisitionMimes": ["application/vnd.comicbook+zip"],
    },
    {
        "code": "G",
        "directory": "G-pdf",
        "libraryName": "G PDF 单 Unit",
        "format": "single-pdf",
        "expectedUnits": 1,
        "expectedPhysicalComics": 1,
        "expectedUnitPageCounts": [6],
        "expectedInternalUnits": False,
        "allowedAcquisitionMimes": ["application/pdf"],
        "allowedSemanticDifferences": {
            "unitCount": "PDF has no portable chapter boundary model and is one Unit.",
            "unitTitles": "PDF exposes 全文 rather than two chapter Units.",
            "unitPageCounts": "The six canonical pages are contiguous in one Unit.",
            "continueUnitIndex": "The only PDF Unit remains the continuation Unit.",
        },
    },
)


def image_bytes(text: str, color: tuple[int, int, int]) -> bytes:
    size = (600, 900)
    image = Image.new("RGB", size, color)
    draw = ImageDraw.Draw(image)
    draw.rectangle((24, 24, 576, 876), outline=(20, 20, 20), width=6)
    draw.text((48, 72), text, fill=(10, 10, 10))
    output = io.BytesIO()
    image.save(output, "JPEG", quality=94, optimize=False, progressive=False)
    return output.getvalue()


CANONICAL_IMAGES = {
    "第001话 起点/cover.jpg": image_bytes("chapter-1-cover", (226, 238, 255)),
    "第001话 起点/001.jpg": image_bytes("chapter-1-page-1", (255, 238, 226)),
    "第001话 起点/002.jpg": image_bytes("chapter-1-page-2", (238, 255, 226)),
    "第002话 回声/cover.jpg": image_bytes("chapter-2-cover", (246, 226, 255)),
    "第002话 回声/001.jpg": image_bytes("chapter-2-page-1", (255, 248, 220)),
    "第002话 回声/002.jpg": image_bytes("chapter-2-page-2", (220, 248, 255)),
}
WORK_COVER = CANONICAL_IMAGES["第001话 起点/cover.jpg"]


def comic_info(title: str, number: str = "") -> bytes:
    number_xml = f"\n  <Number>{number}</Number>" if number else ""
    value = f"""<?xml version="1.0" encoding="utf-8"?>
<ComicInfo>
  <Series>{WORK_TITLE}</Series>
  <Title>{title}</Title>{number_xml}
  <Writer>{WORK_AUTHOR}</Writer>
  <Summary>{WORK_DESCRIPTION}</Summary>
  <LanguageISO>{WORK_LANGUAGE}</LanguageISO>
  <Genre>{WORK_GENRE}</Genre>
</ComicInfo>
"""
    return value.encode("utf-8")


def write_chapter_folder(root: Path, chapter: str, number: str) -> None:
    target = root / chapter
    target.mkdir(parents=True, exist_ok=True)
    (target / "ComicInfo.xml").write_bytes(comic_info(chapter, number))
    for relative_name, payload in CANONICAL_IMAGES.items():
        if relative_name.startswith(chapter + "/"):
            (target / Path(relative_name).name).write_bytes(payload)


def write_chapter_archive(
    target: Path,
    chapter: str,
    number: str,
) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    with ZipFile(target, "w", ZIP_DEFLATED) as archive:
        archive.writestr("ComicInfo.xml", comic_info(chapter, number))
        for relative_name, payload in CANONICAL_IMAGES.items():
            if relative_name.startswith(chapter + "/"):
                archive.writestr(Path(relative_name).name, payload)


def write_large_archive(target: Path, wrapper: str = "") -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    prefix = f"{wrapper}/" if wrapper else ""
    with ZipFile(target, "w", ZIP_DEFLATED) as archive:
        archive.writestr(prefix + "ComicInfo.xml", comic_info(WORK_TITLE))
        archive.writestr(prefix + "cover.jpg", WORK_COVER)
        for chapter, number in CHAPTERS:
            archive.writestr(
                f"{prefix}{chapter}/ComicInfo.xml",
                comic_info(chapter, number),
            )
            for relative_name, payload in CANONICAL_IMAGES.items():
                if relative_name.startswith(chapter + "/"):
                    archive.writestr(prefix + relative_name, payload)


def write_pdf(target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    pages = []
    for relative_name, payload in CANONICAL_IMAGES.items():
        if Path(relative_name).name not in {"cover.jpg", "001.jpg", "002.jpg"}:
            continue
        with Image.open(io.BytesIO(payload)) as source:
            pages.append(source.convert("RGB"))
    pages[0].save(
        target,
        "PDF",
        save_all=True,
        append_images=pages[1:],
        resolution=150,
        title=WORK_TITLE,
        author=WORK_AUTHOR,
        subject=WORK_DESCRIPTION,
        keywords=WORK_GENRE,
    )


def safe_clean_root(root: Path) -> None:
    resolved = root.resolve()
    anchors = {
        Path(resolved.anchor).resolve(),
        Path.home().resolve(),
        Path(__file__).resolve().parents[1],
    }
    if resolved in anchors or len(resolved.parts) < 3:
        raise ValueError(f"refusing to clean unsafe fixture path: {resolved}")
    if resolved.exists():
        shutil.rmtree(resolved)


def canonical_hashes() -> dict[str, str]:
    return {
        name: hashlib.sha256(payload).hexdigest()
        for name, payload in CANONICAL_IMAGES.items()
    }


def average_hash(payload: bytes) -> str:
    with Image.open(io.BytesIO(payload)) as image:
        grayscale = image.convert("L").resize((8, 8))
        pixels = list(grayscale.get_flattened_data())
    average = sum(pixels) / len(pixels)
    bits = "".join("1" if pixel >= average else "0" for pixel in pixels)
    return f"{int(bits, 2):016x}"


def build_fixture(root: Path = DEFAULT_ROOT, clean: bool = True) -> dict:
    root = Path(root).resolve()
    if clean:
        safe_clean_root(root)
    root.mkdir(parents=True, exist_ok=True)

    # A: work/chapter/image folders.
    a_work = root / "A-folder-chapters" / WORK_TITLE
    for chapter, number in CHAPTERS:
        write_chapter_folder(a_work, chapter, number)

    # B: work directory containing multiple CBZ files.
    b_work = root / "B-multi-cbz" / WORK_TITLE
    for chapter, number in CHAPTERS:
        write_chapter_archive(b_work / f"{chapter}.cbz", chapter, number)

    # C: work directory containing multiple ZIP files.
    c_work = root / "C-multi-zip" / WORK_TITLE
    for chapter, number in CHAPTERS:
        write_chapter_archive(c_work / f"{chapter}.zip", chapter, number)

    # D: one large ZIP with chapter directories at archive root.
    write_large_archive(
        root / "D-single-large-zip" / f"{WORK_TITLE}.zip"
    )

    # E: one large CBZ with a single work-name wrapper directory.
    write_large_archive(
        root / "E-single-large-cbz-wrapper" / f"{WORK_TITLE}.cbz",
        wrapper=WORK_TITLE,
    )

    # F: one folder chapter plus one CBZ chapter.
    f_work = root / "F-mixed-folder-archive" / WORK_TITLE
    write_chapter_folder(f_work, CHAPTERS[0][0], CHAPTERS[0][1])
    write_chapter_archive(
        f_work / f"{CHAPTERS[1][0]}.cbz",
        CHAPTERS[1][0],
        CHAPTERS[1][1],
    )

    # G: one PDF containing the same six canonical images in the same order.
    g_root = root / "G-pdf"
    write_pdf(g_root / f"{WORK_TITLE}.pdf")
    metadata_reference = g_root / ".fixture-metadata"
    metadata_reference.mkdir(parents=True, exist_ok=True)
    (metadata_reference / "ComicInfo.xml").write_bytes(comic_info(WORK_TITLE))

    libraries = []
    for variant in PARALLEL_VARIANTS:
        item = dict(variant)
        item.update(
            {
                "workTitle": WORK_TITLE,
                "rootPath": f"/app/comics/fixture/{variant['directory']}",
                "relativeRoot": variant["directory"],
                "expectedTitle": WORK_TITLE,
                "expectedAuthor": WORK_AUTHOR,
                "expectedDescription": WORK_DESCRIPTION,
                "expectedLanguage": WORK_LANGUAGE,
                "expectedGenreTerms": ["平行验收", "统一模型"],
                "expectedChapterTitles": (
                    [chapter for chapter, _ in CHAPTERS]
                    if variant["code"] != "G"
                    else ["全文"]
                ),
                "expectedTotalPages": 6,
                "expectedCoverSha256": hashlib.sha256(WORK_COVER).hexdigest(),
                "expectedCoverAverageHash": average_hash(WORK_COVER),
                "expectedCoverAspectRatio": 600 / 900,
            }
        )
        libraries.append(item)

    manifest = {
        "schemaVersion": 3,
        "expectedWorks": 7,
        "expectedLibraries": 7,
        "comparison": {
            "workTitle": WORK_TITLE,
            "author": WORK_AUTHOR,
            "description": WORK_DESCRIPTION,
            "language": WORK_LANGUAGE,
            "genreTerms": ["平行验收", "统一模型"],
            "chapterTitles": [chapter for chapter, _ in CHAPTERS],
            "unitPageCounts": [3, 3],
            "totalPages": 6,
            "coverSha256": hashlib.sha256(WORK_COVER).hexdigest(),
            "coverAverageHash": average_hash(WORK_COVER),
            "coverAspectRatio": 600 / 900,
            "canonicalPageSha256": canonical_hashes(),
            "canonicalPageAverageHash": {
                name: average_hash(payload)
                for name, payload in CANONICAL_IMAGES.items()
            },
            "canonicalComicInfoSha256": {
                "work": hashlib.sha256(comic_info(WORK_TITLE)).hexdigest(),
                **{
                    chapter: hashlib.sha256(
                        comic_info(chapter, number)
                    ).hexdigest()
                    for chapter, number in CHAPTERS
                },
            },
            "allowedDifferencePaths": [
                "work.id",
                "work.libraryId",
                "work.rootPath",
                "work.seriesId",
                "work.metadataHostType",
                "work.metadataHostId",
                "work.representativeComicId",
                "work.coverComicId",
                "work.fileSize",
                "work.addedAt",
                "work.updatedAt",
                "units[*].id",
                "units[*].comicId",
                "units[*].relativePath",
                "units[*].internalPath",
                "units[*].coverUrl",
                "opds.acquisitionMimes",
                "G.unitCount",
                "G.unitTitles",
                "G.unitPageCounts",
                "G.continueUnitIndex",
                "G.renderedPageSha256",
                "G.renderedCoverSha256",
            ],
        },
        "libraries": libraries,
        "pdfDifference": (
            "Variant G carries the same six page images and matching PDF metadata, "
            "plus a canonical ComicInfo.xml reference under .fixture-metadata, "
            "but PDF is intentionally one Unit because it has no portable chapter "
            "boundary representation."
        ),
    }
    (root / "fixture-manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    return manifest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=DEFAULT_ROOT)
    parser.add_argument(
        "--no-clean",
        action="store_true",
        help="keep existing files instead of recreating the fixture root",
    )
    args = parser.parse_args()
    manifest = build_fixture(args.output, clean=not args.no_clean)
    print(Path(args.output).resolve())
    print(
        f"generated {manifest['expectedLibraries']} parallel libraries "
        f"for {manifest['comparison']['workTitle']}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
