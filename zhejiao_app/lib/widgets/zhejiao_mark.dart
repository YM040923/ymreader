import 'package:flutter/material.dart';

/// 「折角」品牌小标：一页纸、左侧书脊、右上折角。
/// 用在首页左上角；和桌面图标同源，但这里是矢量绘制，24dp 也清晰。
class ZhejiaoMark extends StatelessWidget {
  final double size;

  const ZhejiaoMark({super.key, this.size = 24});

  @override
  Widget build(BuildContext context) {
    return CustomPaint(
      size: Size.square(size),
      painter: const _ZhejiaoMarkPainter(),
    );
  }
}

class _ZhejiaoMarkPainter extends CustomPainter {
  const _ZhejiaoMarkPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.width;
    final left = s * 0.16;
    final top = s * 0.08;
    final right = s * 0.86;
    final bottom = s * 0.92;
    final radius = s * 0.11;
    final fold = s * 0.24;
    final paper = Paint()..color = const Color(0xFFF3EFE9);
    final ink = Paint()..color = const Color(0xFF161616);
    final red = Paint()..color = const Color(0xFFD94B3D);

    final page = Path()
      ..moveTo(left + radius, top)
      ..lineTo(right - fold, top)
      ..lineTo(right, top + fold)
      ..lineTo(right, bottom - radius)
      ..quadraticBezierTo(right, bottom, right - radius, bottom)
      ..lineTo(left + radius, bottom)
      ..quadraticBezierTo(left, bottom, left, bottom - radius)
      ..lineTo(left, top + radius)
      ..quadraticBezierTo(left, top, left + radius, top)
      ..close();
    canvas.drawPath(page, paper);

    final spineWidth = s * 0.10;
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        Rect.fromLTWH(left, top, spineWidth, bottom - top),
        Radius.circular(spineWidth / 2),
      ),
      ink,
    );

    final folded = Path()
      ..moveTo(right - fold, top)
      ..lineTo(right, top + fold)
      ..lineTo(right - fold, top + fold)
      ..close();
    canvas.drawPath(folded, red);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
