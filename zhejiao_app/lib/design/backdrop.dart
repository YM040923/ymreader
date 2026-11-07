import 'package:flutter/material.dart';

/// Fushi 风格的柔光色块：布局之上什么都不画，只在页面底部铺两三个
/// 大半径径向渐变，让暖色背景不是一张死白纸。
class BlobBackdrop extends StatelessWidget {
  const BlobBackdrop({super.key});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return IgnorePointer(
      child: Stack(
        children: [
          Positioned(
            top: -150,
            right: -110,
            child: _Blob(
              size: 340,
              color: colors.primary.withValues(alpha: 0.10),
            ),
          ),
          Positioned(
            top: 280,
            left: -140,
            child: _Blob(
              size: 300,
              color: colors.primaryContainer.withValues(alpha: 0.55),
            ),
          ),
          Positioned(
            bottom: 90,
            right: -100,
            child: _Blob(
              size: 260,
              color: colors.secondaryContainer.withValues(alpha: 0.45),
            ),
          ),
        ],
      ),
    );
  }
}

class _Blob extends StatelessWidget {
  const _Blob({required this.size, required this.color});

  final double size;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: RadialGradient(
          colors: [color, color.withValues(alpha: 0)],
        ),
      ),
    );
  }
}
