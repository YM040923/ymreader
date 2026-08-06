import io
import unittest

from PIL import Image

from scripts.verify_unified_work_container import (
    expected_work_count_after_read,
    opds_work_detail_path,
    readable_image_fingerprint,
    work_stats_path,
)


class VerificationHelpersTest(unittest.TestCase):
    def test_work_stats_path_requests_work_aggregation(self) -> None:
        self.assertEqual(
            work_stats_path(),
            "/api/stats?view=work&includeNovels=true",
        )

    def test_expected_work_count_is_repeatable_when_work_was_already_read(self) -> None:
        self.assertEqual(expected_work_count_after_read(7, True), 7)
        self.assertEqual(expected_work_count_after_read(7, False), 8)

    def test_opds_work_detail_requests_the_full_supported_page(self) -> None:
        self.assertEqual(
            opds_work_detail_path("work_123"),
            "/api/opds/works/work_123?page=1&pageSize=500",
        )

    def test_readable_image_fingerprint_rejects_non_image_payload(self) -> None:
        self.assertIsNone(readable_image_fingerprint(b"not an image"))

    def test_readable_image_fingerprint_accepts_decodable_image(self) -> None:
        image = Image.new("RGB", (12, 18), "red")
        payload = io.BytesIO()
        image.save(payload, format="PNG")

        fingerprint = readable_image_fingerprint(payload.getvalue())

        self.assertIsNotNone(fingerprint)
        self.assertEqual(fingerprint["width"], 12)
        self.assertEqual(fingerprint["height"], 18)


if __name__ == "__main__":
    unittest.main()
