import 'package:flutter/material.dart';

import '../design/theme.dart';
import '../design/tokens.dart';

class PreviewPage extends StatelessWidget {
  const PreviewPage({super.key, required this.onToggleTheme});

  final VoidCallback onToggleTheme;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final width = MediaQuery.sizeOf(context).width;
    final columns = zCoverColumns(width);

    return Scaffold(
      body: Stack(
        children: [
          SingleChildScrollView(
            padding: const EdgeInsets.only(bottom: 120),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 1200),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _Header(onToggleTheme: onToggleTheme),
                      const SizedBox(height: 24),
                      _Hero(compact: width < 700),
                      const SizedBox(height: ZSpace.section),
                      const _SectionTitle('继续阅读', trailing: '全部'),
                      const SizedBox(height: 12),
                      const _ContinueRow(),
                      const SizedBox(height: ZSpace.section),
                      _SectionTitle('书架', trailing: '$columns 列 · 1200dp 封顶'),
                      const SizedBox(height: 12),
                      _Shelf(columns: columns),
                      const SizedBox(height: ZSpace.section),
                      const _SectionTitle('表面层级'),
                      const SizedBox(height: 12),
                      const _SurfaceLadder(),
                      const SizedBox(height: ZSpace.section),
                      const _SectionTitle('字体'),
                      const SizedBox(height: 12),
                      const _TypeSpecimen(),
                      const SizedBox(height: ZSpace.section),
                      const _SectionTitle('组件'),
                      const SizedBox(height: 12),
                      const _Controls(),
                    ],
                  ),
                ),
              ),
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            height: 110,
            child: IgnorePointer(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      colors.surface.withValues(alpha: 0),
                      colors.surface,
                    ],
                  ),
                ),
              ),
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 16,
            child: Center(child: _BottomNav(width: width)),
          ),
        ],
      ),
      backgroundColor: colors.surface,
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.onToggleTheme});

  final VoidCallback onToggleTheme;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final dark = Theme.of(context).brightness == Brightness.dark;
    return Row(
      children: [
        Container(
          width: 32,
          height: 32,
          decoration: BoxDecoration(
            color: colors.primaryContainer,
            borderRadius: BorderRadius.circular(10),
          ),
          alignment: Alignment.center,
          child: Text(
            '折',
            style: text.titleMedium?.copyWith(
              color: colors.onPrimaryContainer,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
        const SizedBox(width: 10),
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '折角',
              style: text.titleLarge?.copyWith(fontWeight: FontWeight.w600),
            ),
            Text(
              'Design preview · M3 Expressive',
              style: text.labelSmall?.copyWith(color: colors.onSurfaceVariant),
            ),
          ],
        ),
        const Spacer(),
        IconButton(
          tooltip: dark ? '切到亮色' : '切到深色',
          onPressed: onToggleTheme,
          icon: Icon(dark ? Icons.light_mode_outlined : Icons.dark_mode_outlined),
        ),
        IconButton(
          tooltip: '搜索',
          onPressed: () {},
          icon: const Icon(Icons.search_rounded),
        ),
      ],
    );
  }
}

class _Hero extends StatelessWidget {
  const _Hero({required this.compact});

  final bool compact;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final title = compact ? text.headlineSmall : text.displaySmall;
    final dark = Theme.of(context).brightness == Brightness.dark;
    final heroStart = dark
        ? Color.lerp(colors.surfaceContainerHigh, colors.primaryContainer, 0.35)!
        : colors.primaryContainer;
    return Container(
      width: double.infinity,
      height: compact ? 220 : 300,
      padding: EdgeInsets.all(compact ? 20 : 28),
      decoration: BoxDecoration(
        borderRadius: ZRadius.groupAll,
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            heroStart,
            colors.surfaceContainerHigh,
          ],
        ),
      ),
      child: Stack(
        children: [
          Positioned(
            right: 0,
            top: 0,
            bottom: 0,
            child: Align(
              alignment: Alignment.centerRight,
              child: _MockCover(
                width: compact ? 120 : 180,
                height: compact ? 168 : 236,
                seed: 0,
                radius: 20,
                monogram: '星',
              ),
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              Text(
                '继续阅读',
                style: text.labelMedium?.copyWith(
                  color: colors.onPrimaryContainer,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 6),
              Text(
                '星海旅人',
                style: title?.copyWith(
                  color: colors.onSurface,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                '第 12 话 · 读到 34%',
                style: text.bodyMedium?.copyWith(
                  color: colors.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              FilledButton.icon(
                onPressed: () {},
                icon: const Icon(Icons.play_arrow_rounded, size: 18),
                label: const Text('继续阅读'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.title, {this.trailing});

  final String title;
  final String? trailing;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Text(
          title,
          style: text.titleLarge?.copyWith(fontWeight: FontWeight.w600),
        ),
        const Spacer(),
        if (trailing != null)
          Text(
            trailing!,
            style: text.labelMedium?.copyWith(
              color: colors.onSurfaceVariant,
            ),
          ),
      ],
    );
  }
}

class _ContinueRow extends StatelessWidget {
  const _ContinueRow();

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final cardWidth = width < 700 ? 220.0 : 280.0;
    return SizedBox(
      height: 96,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: 4,
        separatorBuilder: (_, _) => const SizedBox(width: 12),
        itemBuilder: (context, index) => SizedBox(
          width: cardWidth,
          child: _ContinueCard(index: index),
        ),
      ),
    );
  }
}

class _ContinueCard extends StatelessWidget {
  const _ContinueCard({required this.index});

  final int index;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.surfaceContainerLow,
        borderRadius: ZRadius.groupAll,
      ),
      child: Row(
        children: [
          _MockCover(
            width: 52,
            height: 64,
            seed: index + 1,
            radius: 8,
            monogram: _titles[(index + 1) % _titles.length].substring(0, 1),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _titles[(index + 1) % _titles.length],
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: text.titleSmall?.copyWith(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 4),
                Text(
                  '第 ${index + 3} 话 · ${32 + index * 11}%',
                  style: text.labelMedium?.copyWith(
                    color: colors.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 8),
                ClipRRect(
                  borderRadius: BorderRadius.circular(2),
                  child: LinearProgressIndicator(
                    value: 0.32 + index * 0.11,
                    minHeight: 3,
                    backgroundColor: colors.surfaceContainerHighest,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Shelf extends StatelessWidget {
  const _Shelf({required this.columns});

  final int columns;

  @override
  Widget build(BuildContext context) {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: columns * 2,
      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: columns,
        crossAxisSpacing: columns > 2 ? 16 : 8,
        mainAxisSpacing: columns > 2 ? 24 : 16,
        childAspectRatio: 0.62,
      ),
      itemBuilder: (context, index) => _ShelfCard(index: index),
    );
  }
}

class _ShelfCard extends StatelessWidget {
  const _ShelfCard({required this.index});

  final int index;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: AspectRatio(
            aspectRatio: 3 / 4,
            child: _MockCover(
              width: double.infinity,
              height: double.infinity,
              seed: index,
              radius: 12,
              monogram: _titles[index % _titles.length].substring(0, 1),
            ),
          ),
        ),
        const SizedBox(height: 8),
        Text(
          _titles[index % _titles.length],
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: text.titleSmall?.copyWith(fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 2),
        Text(
          '第 ${10 + index} 话 · 连载中',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: text.labelSmall?.copyWith(color: colors.onSurfaceVariant),
        ),
      ],
    );
  }
}

class _MockCover extends StatelessWidget {
  const _MockCover({
    required this.width,
    required this.height,
    required this.seed,
    required this.radius,
    this.monogram,
    this.label,
  });

  final double width;
  final double height;
  final int seed;
  final double radius;
  final String? monogram;
  final String? label;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final palettes = <List<Color>>[
      [colors.primary, colors.tertiary],
      [colors.tertiary, colors.secondary],
      [colors.secondary, colors.primary],
      [colors.error, colors.tertiary],
      [colors.primary, colors.secondary],
      [colors.tertiary, colors.error],
    ];
    final palette = palettes[seed % palettes.length];
    return ClipRRect(
      borderRadius: BorderRadius.circular(radius),
      child: Container(
        width: width,
        height: height,
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [
              palette.first.withValues(alpha: 0.82),
              palette.last.withValues(alpha: 0.55),
            ],
          ),
        ),
        child: label == null
            ? (monogram == null
                ? null
                : Center(
                    child: Text(
                      monogram!,
                      style: TextStyle(
                        fontSize: (width.isFinite ? width * 0.44 : 64)
                            .clamp(24.0, 72.0)
                            .toDouble(),
                        fontWeight: FontWeight.w700,
                        color: Colors.white.withValues(alpha: 0.30),
                      ),
                    ),
                  ))
            : Align(
                alignment: Alignment.bottomLeft,
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: colors.surface.withValues(alpha: 0.72),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      label!,
                      style: Theme.of(context).textTheme.labelSmall?.copyWith(
                            color: colors.onSurface,
                            fontWeight: FontWeight.w600,
                          ),
                    ),
                  ),
                ),
              ),
      ),
    );
  }
}

class _SurfaceLadder extends StatelessWidget {
  const _SurfaceLadder();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final items = <(String, Color)>[
      ('surfaceContainerLowest', colors.surfaceContainerLowest),
      ('surface', colors.surface),
      ('surfaceContainerLow', colors.surfaceContainerLow),
      ('surfaceContainer', colors.surfaceContainer),
      ('surfaceContainerHigh', colors.surfaceContainerHigh),
      ('surfaceContainerHighest', colors.surfaceContainerHighest),
    ];
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = constraints.maxWidth >= 900 ? 6 : 2;
        return GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: items.length,
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: columns,
            crossAxisSpacing: 12,
            mainAxisSpacing: 12,
            childAspectRatio: columns == 6 ? 1.55 : 2.2,
          ),
          itemBuilder: (context, index) {
            final item = items[index];
            return Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: item.$2,
                borderRadius: ZRadius.cardAll,
                border: Border.all(color: colors.outlineVariant),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  Text(
                    item.$1,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: text.labelSmall?.copyWith(
                      color: colors.onSurface,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
            );
          },
        );
      },
    );
  }
}

class _TypeSpecimen extends StatelessWidget {
  const _TypeSpecimen();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final rows = <(String, TextStyle?)>[
      ('displaySmall 36', text.displaySmall),
      ('headlineSmall 24', text.headlineSmall),
      ('titleLarge 22', text.titleLarge),
      ('titleMedium 16', text.titleMedium),
      ('bodyLarge 16', text.bodyLarge),
      ('bodyMedium 14', text.bodyMedium),
      ('labelMedium 12', text.labelMedium),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final row in rows)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.baseline,
              textBaseline: TextBaseline.alphabetic,
              children: [
                SizedBox(
                  width: 150,
                  child: Text(
                    row.$1,
                    style: text.labelSmall?.copyWith(
                      color: colors.onSurfaceVariant,
                    ),
                  ),
                ),
                Expanded(
                  child: Text(
                    '折角 · 漫画阅读器 Aa 0123',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: row.$2,
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

class _Controls extends StatelessWidget {
  const _Controls();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 10,
          runSpacing: 10,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            FilledButton(onPressed: () {}, child: const Text('主按钮')),
            OutlinedButton(onPressed: () {}, child: const Text('次按钮')),
            TextButton(onPressed: () {}, child: const Text('文字按钮')),
            FilterChip(
              selected: true,
              onSelected: (_) {},
              label: const Text('已选中'),
            ),
            const FilterChip(
              selected: false,
              onSelected: null,
              label: Text('未选中'),
            ),
            Switch(value: true, onChanged: (_) {}),
          ],
        ),
        const SizedBox(height: 16),
        SizedBox(
          width: 360,
          child: TextField(
            decoration: InputDecoration(
              hintText: '搜索漫画、作者或目录名称',
              prefixIcon: Icon(
                Icons.search_rounded,
                size: 20,
                color: colors.onSurfaceVariant,
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),
        SizedBox(
          width: 360,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            minVerticalPadding: 0,
            leading: Icon(Icons.auto_stories_outlined, color: colors.primary),
            title: const Text('第 12 话 · 更依赖我一些'),
            subtitle: const Text('读到 34%'),
            trailing: const Icon(Icons.chevron_right_rounded),
          ),
        ),
      ],
    );
  }
}

class _BottomNav extends StatelessWidget {
  const _BottomNav({required this.width});

  final double width;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final glass = context.zGlass;
    final maxWidth = width < 640 ? width - 32 : 560.0;
    final items = <(IconData, IconData, String)>[
      (Icons.home_outlined, Icons.home_rounded, '首页'),
      (Icons.explore_outlined, Icons.explore_rounded, '发现'),
      (Icons.collections_bookmark_outlined, Icons.collections_bookmark_rounded,
          '记录'),
      (Icons.person_outline_rounded, Icons.person_rounded, '我的'),
    ];
    return Container(
      width: maxWidth,
      height: 64,
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: glass.background,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: glass.border),
      ),
      child: Row(
        children: [
          for (var i = 0; i < items.length; i++)
            Expanded(
              child: AnimatedContainer(
                duration: ZMotion.short,
                curve: ZMotion.standard,
                decoration: BoxDecoration(
                  color: i == 0 ? colors.secondaryContainer : Colors.transparent,
                  borderRadius: BorderRadius.circular(18),
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      i == 0 ? items[i].$2 : items[i].$1,
                      size: 22,
                      color: i == 0 ? colors.primary : colors.onSurfaceVariant,
                    ),
                    const SizedBox(height: 2),
                    Text(
                      items[i].$3,
                      style: Theme.of(context).textTheme.labelSmall?.copyWith(
                            color: i == 0
                                ? colors.primary
                                : colors.onSurfaceVariant,
                            fontWeight:
                                i == 0 ? FontWeight.w700 : FontWeight.w500,
                          ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}

const _titles = <String>[
  '星海旅人',
  '雨夜书店',
  '长夜将明',
  '风与旧信',
  '第七码头',
  '白昼回声',
  '雾中列车',
  '南方来信',
  '山海食堂',
  '最后一场雨',
  '银河维修站',
  '春分之后',
];
