import 'package:flutter/material.dart';

/// 36×20 的轻量开关。行本身负责点击，这里只画状态，
/// 避免 Material 3 的 52×32 开关把 52dp 的设置行撑开。
class AppSwitch extends StatelessWidget {
  const AppSwitch({
    super.key,
    required this.value,
    this.enabled = true,
    this.activeColor,
    this.inactiveColor,
  });

  final bool value;
  final bool enabled;
  final Color? activeColor;
  final Color? inactiveColor;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final track = value
        ? (activeColor ?? colors.primary)
        : (inactiveColor ?? colors.outlineVariant);
    return Semantics(
      checked: value,
      enabled: enabled,
      child: Opacity(
        opacity: enabled ? 1 : 0.38,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 160),
          curve: Curves.easeOut,
          width: 36,
          height: 20,
          padding: const EdgeInsets.all(2),
          decoration: BoxDecoration(
            color: track,
            borderRadius: BorderRadius.circular(10),
          ),
          child: AnimatedAlign(
            duration: const Duration(milliseconds: 160),
            curve: Curves.easeOut,
            alignment: value ? Alignment.centerRight : Alignment.centerLeft,
            child: const DecoratedBox(
              decoration: BoxDecoration(
                color: Colors.white,
                shape: BoxShape.circle,
              ),
              child: SizedBox(width: 16, height: 16),
            ),
          ),
        ),
      ),
    );
  }
}
