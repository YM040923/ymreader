import 'package:flutter/material.dart';


/// M0：按 Fushi 的真实书架页复刻，而不是只摆一套 token。
///
/// 桌面：左侧导航栏 + 书架标题/搜索/筛选/新建 + 筛选胶囊 + 继续阅读横幅 + 6 列书架。
/// 手机：标题栏 + 继续阅读横幅 + 网格/列表/合集分段 + 3 列书架 + 底部五项导航。
class PreviewPage extends StatelessWidget {
  const PreviewPage({super.key, required this.onToggleTheme});

  final VoidCallback onToggleTheme;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return LayoutBuilder(
      builder: (context, constraints) {
        final desktop = constraints.maxWidth >= 900;
        return Scaffold(
          backgroundColor:
              desktop ? colors.surface : colors.surfaceContainerLowest,
          body: desktop
              ? Row(
                  children: [
                    _DesktopRail(onToggleTheme: onToggleTheme),
                    Expanded(child: _DesktopLibrary()),
                  ],
                )
              : const _MobileLibrary(),
          bottomNavigationBar: desktop ? null : const _MobileNav(),
        );
      },
    );
  }
}

// ---------------------------------------------------------------------------
// 桌面
// ---------------------------------------------------------------------------

class _DesktopRail extends StatelessWidget {
  const _DesktopRail({required this.onToggleTheme});

  final VoidCallback onToggleTheme;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final items = <(IconData, IconData, String)>[
      (Icons.home_outlined, Icons.home_rounded, '首页'),
      (Icons.menu_book_outlined, Icons.menu_book_rounded, '书架'),
      (Icons.explore_outlined, Icons.explore_rounded, '发现'),
      (Icons.history, Icons.history_rounded, '记录'),
      (Icons.bookmark_border_rounded, Icons.bookmark_rounded, '收藏'),
      (Icons.settings_outlined, Icons.settings_rounded, '设置'),
    ];
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
            for (var i = 0; i < items.length; i++) ...[
              _RailItem(
                icon: items[i].$1,
                activeIcon: items[i].$2,
                label: items[i].$3,
                selected: i == 1,
              ),
              const SizedBox(height: 10),
            ],
            const Spacer(),
            IconButton(
              tooltip: Theme.of(context).brightness == Brightness.dark
                  ? '切到亮色'
                  : '切到深色',
              onPressed: onToggleTheme,
              icon: Icon(
                Theme.of(context).brightness == Brightness.dark
                    ? Icons.light_mode_outlined
                    : Icons.dark_mode_outlined,
                color: colors.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 12),
          ],
        ),
      ),
    );
  }
}

class _RailItem extends StatelessWidget {
  const _RailItem({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.selected,
  });

  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool selected;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Container(
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
            selected ? activeIcon : icon,
            size: 24,
            color: selected ? colors.primary : colors.onSurfaceVariant,
          ),
          const SizedBox(height: 4),
          Text(
            label,
            style: text.labelMedium?.copyWith(
              color: selected ? colors.primary : colors.onSurfaceVariant,
              fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }
}

class _DesktopLibrary extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(40, 24, 40, 40),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                '书架',
                style: text.displaySmall?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(width: 10),
              Text(
                '128 本',
                style: text.labelMedium?.copyWith(
                  color: colors.onSurfaceVariant,
                ),
              ),
              const Spacer(),
              const _SearchField(width: 360),
              const SizedBox(width: 12),
              _CircleButton(
                icon: Icons.tune_rounded,
                background: colors.primaryContainer,
                foreground: colors.primary,
                tooltip: '筛选',
              ),
              const SizedBox(width: 8),
              _CircleButton(
                icon: Icons.add_rounded,
                background: colors.primary,
                foreground: colors.onPrimary,
                tooltip: '新建',
              ),
            ],
          ),
          const SizedBox(height: 22),
          const Row(
            children: [
              _FilterPill('全部', selected: true),
              SizedBox(width: 8),
              _FilterPill('在读'),
              SizedBox(width: 8),
              _FilterPill('已读完'),
              SizedBox(width: 8),
              _FilterPill('有声书'),
              SizedBox(width: 8),
              _FilterPill('#轻小说'),
            ],
          ),
          const SizedBox(height: 24),
          const _ContinueBanner(desktop: true),
          const SizedBox(height: 30),
          Text(
            '最近添加',
            style: text.titleLarge?.copyWith(fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 16),
          const _BookGrid(columns: 6, count: 12),
        ],
      ),
    );
  }
}

class _SearchField extends StatelessWidget {
  const _SearchField({required this.width});

  final double width;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return SizedBox(
      width: width,
      height: 52,
      child: TextField(
        readOnly: true,
        style: text.bodyMedium,
        decoration: InputDecoration(
          isDense: true,
          filled: true,
          fillColor: colors.surfaceContainerHigh,
          hintText: '搜索书名、作者',
          hintStyle: text.bodyMedium?.copyWith(
            color: colors.onSurfaceVariant,
          ),
          prefixIcon: Icon(
            Icons.search_rounded,
            size: 22,
            color: colors.onSurfaceVariant,
          ),
          contentPadding: const EdgeInsets.symmetric(vertical: 16),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(26),
            borderSide: BorderSide.none,
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(26),
            borderSide: BorderSide.none,
          ),
        ),
      ),
    );
  }
}

class _CircleButton extends StatelessWidget {
  const _CircleButton({
    required this.icon,
    required this.background,
    required this.foreground,
    required this.tooltip,
  });

  final IconData icon;
  final Color background;
  final Color foreground;
  final String tooltip;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: background,
        shape: const CircleBorder(),
        child: InkWell(
          customBorder: const CircleBorder(),
          onTap: () {},
          child: SizedBox(
            width: 48,
            height: 48,
            child: Icon(icon, size: 22, color: foreground),
          ),
        ),
      ),
    );
  }
}

class _FilterPill extends StatelessWidget {
  const _FilterPill(this.label, {this.selected = false});

  final String label;
  final bool selected;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Container(
      height: 40,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: selected ? colors.primaryContainer : colors.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: selected
            ? null
            : Border.all(color: colors.outlineVariant, width: 1),
      ),
      child: Text(
        label,
        style: text.labelLarge?.copyWith(
          color: selected ? colors.primary : colors.onSurface,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// 继续阅读 + 书架
// ---------------------------------------------------------------------------

class _ContinueBanner extends StatelessWidget {
  const _ContinueBanner({required this.desktop});

  final bool desktop;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final book = _books[2];
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(desktop ? 20 : 16),
      decoration: BoxDecoration(
        color: colors.secondaryContainer,
        borderRadius: BorderRadius.circular(desktop ? 20 : 16),
      ),
      child: Row(
        children: [
          SizedBox(
            width: desktop ? 72 : 64,
            height: desktop ? 96 : 88,
            child: _TypographicCover(
              title: book.title,
              progress: book.progress,
              palette: book.palette,
            ),
          ),
          SizedBox(width: desktop ? 20 : 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '继续阅读',
                  style: text.labelMedium?.copyWith(
                    color: colors.primary,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  book.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: text.titleLarge?.copyWith(
                    fontSize: desktop ? 22 : 20,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                SizedBox(height: desktop ? 12 : 10),
                ClipRRect(
                  borderRadius: BorderRadius.circular(2),
                  child: LinearProgressIndicator(
                    value: book.progress,
                    minHeight: 4,
                    color: colors.primary,
                    backgroundColor: colors.onSurface.withValues(alpha: 0.12),
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  '第 7 章 · 剩余约 42 分钟',
                  style: text.labelMedium?.copyWith(
                    color: colors.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          if (desktop) ...[
            const SizedBox(width: 24),
            FilledButton.icon(
              onPressed: () {},
              style: FilledButton.styleFrom(
                backgroundColor: colors.primary,
                foregroundColor: colors.onPrimary,
                minimumSize: const Size(104, 48),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(24),
                ),
              ),
              icon: const Icon(Icons.play_arrow_rounded, size: 20),
              label: const Text('继续'),
            ),
          ],
        ],
      ),
    );
  }
}

class _BookGrid extends StatelessWidget {
  const _BookGrid({required this.columns, required this.count});

  final int columns;
  final int count;

  @override
  Widget build(BuildContext context) {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: count,
      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: columns,
        crossAxisSpacing: columns >= 4 ? 24 : 12,
        mainAxisSpacing: columns >= 4 ? 32 : 20,
        childAspectRatio: columns >= 4 ? 0.62 : 0.58,
      ),
      itemBuilder: (context, index) => _BookCard(book: _books[index % _books.length]),
    );
  }
}

class _BookCard extends StatelessWidget {
  const _BookCard({required this.book});

  final _Book book;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: _TypographicCover(
            title: book.title,
            progress: book.progress,
            palette: book.palette,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          book.title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: text.titleSmall?.copyWith(
            fontSize: 15,
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 2),
        Text(
          book.subtitle,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: text.labelMedium?.copyWith(color: colors.onSurfaceVariant),
        ),
      ],
    );
  }
}

/// Fushi 参考图里的占位封面：左上标题、右下大字符、底部细进度条。
class _TypographicCover extends StatelessWidget {
  const _TypographicCover({
    required this.title,
    required this.progress,
    required this.palette,
  });

  final String title;
  final double progress;
  final int palette;

  @override
  Widget build(BuildContext context) {
    final pairs = <List<Color>>[
      [const Color(0xFF5FB39A), const Color(0xFF2E7D6B)],
      [const Color(0xFFC2A24E), const Color(0xFF8A6B2E)],
      [const Color(0xFFD46A5E), const Color(0xFF9F3E36)],
      [const Color(0xFF6DBE6B), const Color(0xFF3E8E4A)],
      [const Color(0xFF5B8FC9), const Color(0xFF2E5C8B)],
      [const Color(0xFFB05FAE), const Color(0xFF6B2E7A)],
      [const Color(0xFFD08A4E), const Color(0xFF8B4E2E)],
      [const Color(0xFF8F6BC9), const Color(0xFF5A3E8B)],
      [const Color(0xFF4EA6C9), const Color(0xFF2E6B8B)],
      [const Color(0xFF4E9E8F), const Color(0xFF2E6B5E)],
      [const Color(0xFFC9A34E), const Color(0xFF8B6B2E)],
      [const Color(0xFFC96B8F), const Color(0xFF8B2E5A)],
    ];
    final colorsPair = pairs[palette % pairs.length];
    final first = title.characters.first;
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        return ClipRRect(
          borderRadius: BorderRadius.circular(12),
          child: Stack(
            fit: StackFit.expand,
            children: [
              DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: colorsPair,
                  ),
                ),
              ),
              Positioned(
                left: 12,
                top: 10,
                right: 12,
                child: Text(
                  title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    height: 1.25,
                  ),
                ),
              ),
              Positioned(
                right: -width * 0.04,
                bottom: -width * 0.12,
                child: Text(
                  first,
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.16),
                    fontSize: width * 0.55,
                    fontWeight: FontWeight.w700,
                    height: 1,
                  ),
                ),
              ),
              if (progress > 0)
                Positioned(
                  left: 10,
                  right: 10,
                  bottom: 10,
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(2),
                    child: LinearProgressIndicator(
                      value: progress,
                      minHeight: 3,
                      color: Colors.white.withValues(alpha: 0.92),
                      backgroundColor: Colors.white.withValues(alpha: 0.26),
                    ),
                  ),
                )
              else
                Positioned(
                  right: 10,
                  bottom: 10,
                  child: Container(
                    width: 6,
                    height: 6,
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: 0.72),
                      shape: BoxShape.circle,
                    ),
                  ),
                ),
            ],
          ),
        );
      },
    );
  }
}

// ---------------------------------------------------------------------------
// 手机
// ---------------------------------------------------------------------------

class _MobileLibrary extends StatelessWidget {
  const _MobileLibrary();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return SafeArea(
      bottom: false,
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  '书架',
                  style: text.displaySmall?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const Spacer(),
                Icon(
                  Icons.search_rounded,
                  size: 26,
                  color: colors.onSurface,
                ),
                const SizedBox(width: 18),
                Icon(
                  Icons.more_vert_rounded,
                  size: 26,
                  color: colors.onSurface,
                ),
              ],
            ),
            const SizedBox(height: 18),
            const _ContinueBanner(desktop: false),
            const SizedBox(height: 18),
            const _Segment(),
            const SizedBox(height: 16),
            const _BookGrid(columns: 3, count: 9),
          ],
        ),
      ),
    );
  }
}

class _Segment extends StatelessWidget {
  const _Segment();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    const items = ['网格', '列表', '合集'];
    return Container(
      height: 44,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(22),
        border: Border.all(color: colors.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: Row(
        children: [
          for (var i = 0; i < items.length; i++)
            Expanded(
              child: Container(
                alignment: Alignment.center,
                color: i == 0 ? colors.primaryContainer : Colors.transparent,
                child: Text(
                  items[i],
                  style: text.labelLarge?.copyWith(
                    fontSize: 15,
                    color: i == 0 ? colors.primary : colors.onSurface,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _MobileNav extends StatelessWidget {
  const _MobileNav();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final items = <(IconData, IconData, String)>[
      (Icons.home_outlined, Icons.home_rounded, '首页'),
      (Icons.menu_book_outlined, Icons.menu_book_rounded, '书架'),
      (Icons.explore_outlined, Icons.explore_rounded, '发现'),
      (Icons.history, Icons.history_rounded, '记录'),
      (Icons.person_outline_rounded, Icons.person_rounded, '我的'),
    ];
    return Container(
      decoration: BoxDecoration(
        color: colors.surfaceContainerLowest,
        border: Border(top: BorderSide(color: colors.outlineVariant)),
      ),
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: 72,
          child: Row(
            children: [
              for (var i = 0; i < items.length; i++)
                Expanded(
                  child: Center(
                    child: Container(
                      width: 64,
                      height: 56,
                      decoration: BoxDecoration(
                        color: i == 1
                            ? colors.primaryContainer
                            : Colors.transparent,
                        borderRadius: BorderRadius.circular(16),
                      ),
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(
                            i == 1 ? items[i].$2 : items[i].$1,
                            size: 24,
                            color: i == 1 ? colors.primary : colors.onSurfaceVariant,
                          ),
                          const SizedBox(height: 3),
                          Text(
                            items[i].$3,
                            style: text.labelSmall?.copyWith(
                              color: i == 1
                                  ? colors.primary
                                  : colors.onSurfaceVariant,
                              fontWeight:
                                  i == 1 ? FontWeight.w700 : FontWeight.w500,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// 示例数据
// ---------------------------------------------------------------------------

class _Book {
  const _Book(this.title, this.subtitle, this.progress, this.palette);

  final String title;
  final String subtitle;
  final double progress;
  final int palette;
}

const _books = <_Book>[
  _Book('久保同学不放过我', '雪森宁宁 · 已读完', 1, 0),
  _Book('大王饶命', '会说话的肘子 · 18%', 0.18, 1),
  _Book('从姑获鸟开始', '活儿该 · 40%', 0.40, 2),
  _Book('仙子请听我解释', '徐二家的猫 · 85%', 0.85, 3),
  _Book('义妹生活', '三河ごーすと · 5%', 0.05, 4),
  _Book('女主播攻略', '佚名 · 32%', 0.32, 5),
  _Book('医统·乱世', '尝谕 · 未读', 0, 6),
  _Book('我的守护女友', '烟斗老哥 · 12%', 0.12, 7),
  _Book('大反派名单', '七只跳蚤 · 未读', 0, 8),
  _Book('氪金大佬', '小仙女的猫 · 60%', 0.60, 9),
  _Book('政宗君的复仇', '竹冈叶月 · 22%', 0.22, 10),
  _Book('女友成堆', '弘幸 · 8%', 0.08, 11),
];
