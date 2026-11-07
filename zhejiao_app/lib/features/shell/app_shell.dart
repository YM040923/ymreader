import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../design/backdrop.dart';
import '../../design/tokens.dart';
import '../../widgets/phosphor_icons.dart';

/// 新客户端的 Fushi 式外壳：
/// 桌面左侧导航栏，手机底部五项（这里先四项）固定导航。
/// 保留旧页面的 `shellBarClearance` 约定，让已有滚动页不用改。
double shellBarClearance(BuildContext context) {
  if (MediaQuery.sizeOf(context).width >= 900) return 0;
  return 88 + MediaQuery.paddingOf(context).bottom;
}

class AppShell extends StatelessWidget {
  final Widget child;
  const AppShell({super.key, required this.child});

  static int _indexOf(BuildContext context) {
    final location = GoRouterState.of(context).matchedLocation;
    if (location.startsWith('/discover')) return 1;
    if (location.startsWith('/records')) return 2;
    if (location.startsWith('/settings')) return 3;
    return 0;
  }

  void _go(BuildContext context, int index) {
    switch (index) {
      case 0:
        context.go('/');
      case 1:
        context.go('/discover');
      case 2:
        context.go('/records');
      case 3:
        context.go('/settings');
    }
  }

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final desktop = width >= 900;
    final index = _indexOf(context);
    if (desktop) {
      return Row(
        children: [
          _DesktopRail(index: index, onSelected: (i) => _go(context, i)),
          Expanded(child: child),
        ],
      );
    }
    return ColoredBox(
      color: Theme.of(context).scaffoldBackgroundColor,
      child: Stack(
        children: [
          const Positioned.fill(child: BlobBackdrop()),
          Positioned.fill(child: child),
          Positioned(
          left: 16,
          right: 16,
          bottom: 12 + MediaQuery.paddingOf(context).bottom,
          child: _MobileNav(
            index: index,
            onSelected: (i) => _go(context, i),
          ),
          ),
        ],
      ),
    );
  }
}

class _NavItem {
  const _NavItem(this.icon, this.activeIcon, this.label);

  final IconData icon;
  final IconData activeIcon;
  final String label;
}

const _items = <_NavItem>[
  _NavItem(PhosphorIconsRegular.house, PhosphorIconsFill.house, '首页'),
  _NavItem(PhosphorIconsRegular.compass, PhosphorIconsFill.compass, '发现'),
  _NavItem(PhosphorIconsRegular.books, PhosphorIconsFill.books, '记录'),
  _NavItem(PhosphorIconsRegular.user, PhosphorIconsFill.user, '我的'),
];

class _DesktopRail extends StatelessWidget {
  const _DesktopRail({required this.index, required this.onSelected});

  final int index;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Container(
      width: 120,
      decoration: BoxDecoration(
        color: colors.surfaceContainerLowest,
        border: Border(
          right: BorderSide(color: colors.outlineVariant.withValues(alpha: 0.6)),
        ),
      ),
      child: SafeArea(
        right: false,
        child: Column(
          children: [
            const SizedBox(height: 24),
            Container(
              width: 40,
              height: 40,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: colors.primary,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Text(
                '折',
                style: TextStyle(
                  color: colors.onPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
            const SizedBox(height: 36),
            for (var i = 0; i < _items.length; i++) ...[
              _RailItem(
                item: _items[i],
                selected: i == index,
                onTap: () => onSelected(i),
              ),
              const SizedBox(height: 10),
            ],
          ],
        ),
      ),
    );
  }
}

class _RailItem extends StatelessWidget {
  const _RailItem({
    required this.item,
    required this.selected,
    required this.onTap,
  });

  final _NavItem item;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Container(
        width: 88,
        height: 64,
        decoration: BoxDecoration(
          color: selected ? colors.primaryContainer : Colors.transparent,
          borderRadius: BorderRadius.circular(16),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              selected ? item.activeIcon : item.icon,
              size: 24,
              color: selected ? colors.primary : colors.onSurfaceVariant,
            ),
            const SizedBox(height: 4),
            Text(
              item.label,
              style: text.labelMedium?.copyWith(
                color: selected ? colors.primary : colors.onSurfaceVariant,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _MobileNav extends StatelessWidget {
  const _MobileNav({required this.index, required this.onSelected});

  final int index;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Container(
      height: 64,
      padding: const EdgeInsets.symmetric(horizontal: 10),
      decoration: BoxDecoration(
        color: colors.tertiaryContainer,
        borderRadius: BorderRadius.circular(32),
        boxShadow: [
          BoxShadow(
            color: colors.shadow.withValues(alpha: 0.10),
            blurRadius: 24,
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Row(
        children: [
          for (var i = 0; i < _items.length; i++)
            Expanded(
              child: Tooltip(
                message: _items[i].label,
                child: InkWell(
                  onTap: () => onSelected(i),
                  borderRadius: BorderRadius.circular(24),
                  child: Center(
                    child: AnimatedContainer(
                      duration: ZMotion.short,
                      curve: ZMotion.standard,
                      width: 44,
                      height: 44,
                      decoration: BoxDecoration(
                        color: i == index
                            ? colors.primary
                            : Colors.transparent,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        i == index ? _items[i].activeIcon : _items[i].icon,
                        size: 23,
                        color: i == index
                            ? colors.onPrimary
                            : colors.onTertiaryContainer,
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
