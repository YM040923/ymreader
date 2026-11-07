part of 'home_screen.dart';

class _LibraryHeader extends StatelessWidget {
  const _LibraryHeader({
    required this.horizontal,
    required this.desktop,
    required this.total,
    required this.directoryMode,
    required this.canBrowseDirectory,
    required this.onSearch,
    required this.onDirectory,
    required this.onUpload,
    required this.onMenu,
    required this.viewMode,
  });

  final double horizontal;
  final bool desktop;
  final int total;
  final bool directoryMode;
  final bool canBrowseDirectory;
  final VoidCallback onSearch;
  final VoidCallback? onDirectory;
  final VoidCallback onUpload;
  final ValueChanged<String> onMenu;
  final ViewMode viewMode;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Padding(
      padding: EdgeInsets.fromLTRB(horizontal, 24, horizontal, 0),
      child: Row(
        children: [
          Text(
            '书架',
            style: text.displaySmall?.copyWith(fontWeight: FontWeight.w700),
          ),
          const SizedBox(width: 10),
          Text(
            '$total 本',
            style: text.labelMedium?.copyWith(color: colors.onSurfaceVariant),
          ),
          const Spacer(),
          if (desktop)
            _SearchBox(onTap: onSearch)
          else
            IconButton(
              tooltip: '搜索',
              onPressed: onSearch,
              icon: const Icon(PhosphorIconsRegular.magnifyingGlass, size: 24),
            ),
          IconButton(
            tooltip: directoryMode ? '退出目录浏览' : '按目录浏览',
            onPressed: canBrowseDirectory ? onDirectory : null,
            icon: Icon(
              directoryMode
                  ? PhosphorIconsFill.folderSimple
                  : PhosphorIconsRegular.folderSimple,
              size: 23,
              color: directoryMode ? colors.primary : null,
            ),
          ),
          PopupMenuButton<String>(
            tooltip: '视图与排序',
            icon: const Icon(PhosphorIconsRegular.dotsThree, size: 23),
            onSelected: onMenu,
            itemBuilder: (_) => [
              PopupMenuItem(
                value: 'view',
                child: Text(viewMode == ViewMode.grid ? '列表视图' : '封面墙'),
              ),
              const PopupMenuDivider(),
              const PopupMenuItem(value: 'title', child: Text('按标题')),
              const PopupMenuItem(value: 'addedAt', child: Text('最近添加')),
              const PopupMenuItem(value: 'lastReadAt', child: Text('最近阅读')),
              const PopupMenuItem(value: 'rating', child: Text('评分')),
            ],
          ),
          if (desktop) ...[
            const SizedBox(width: 8),
            _HeaderCircle(
              icon: Icons.add_rounded,
              background: colors.primary,
              foreground: colors.onPrimary,
              tooltip: '导入',
              onTap: onUpload,
            ),
          ],
        ],
      ),
    );
  }
}

class _SearchBox extends StatelessWidget {
  const _SearchBox({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 360,
        height: 52,
        padding: const EdgeInsets.symmetric(horizontal: 18),
        decoration: BoxDecoration(
          color: colors.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(26),
        ),
        child: Row(
          children: [
            Icon(
              PhosphorIconsRegular.magnifyingGlass,
              size: 21,
              color: colors.onSurfaceVariant,
            ),
            const SizedBox(width: 10),
            Text(
              '搜索书名、作者',
              style: text.bodyMedium?.copyWith(color: colors.onSurfaceVariant),
            ),
          ],
        ),
      ),
    );
  }
}

class _HeaderCircle extends StatelessWidget {
  const _HeaderCircle({
    required this.icon,
    required this.background,
    required this.foreground,
    required this.tooltip,
    required this.onTap,
  });

  final IconData icon;
  final Color background;
  final Color foreground;
  final String tooltip;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: background,
        shape: const CircleBorder(),
        child: InkWell(
          customBorder: const CircleBorder(),
          onTap: onTap,
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
  const _FilterPill({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: Material(
        color:
            selected ? colors.primaryContainer : colors.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          borderRadius: BorderRadius.circular(8),
          onTap: onTap,
          child: Container(
            height: 40,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            alignment: Alignment.center,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(8),
              border:
                  selected ? null : Border.all(color: colors.outlineVariant),
            ),
            child: Text(
              label,
              style: text.labelLarge?.copyWith(
                color: selected ? colors.primary : colors.onSurface,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _FushiContinueBanner extends ConsumerStatefulWidget {
  const _FushiContinueBanner();

  @override
  ConsumerState<_FushiContinueBanner> createState() =>
      _FushiContinueBannerState();
}

class _FushiContinueBannerState extends ConsumerState<_FushiContinueBanner> {
  Work? _work;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final response = await ref.read(workApiProvider).listWorks(
            const WorkQuery(
              pageSize: 12,
              sortBy: 'lastReadAt',
              sortOrder: 'desc',
            ),
          );
      if (!mounted) return;
      Work? next;
      for (final work in response.works) {
        if (work.hasReadingProgress) {
          next = work;
          break;
        }
      }
      setState(() => _work = next);
    } catch (_) {
      if (mounted) setState(() => _work = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    final work = _work;
    if (work == null) return const SizedBox.shrink();
    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final serverUrl = ref.watch(authProvider).serverUrl;
    final desktop = MediaQuery.sizeOf(context).width >= 900;
    final target = work.continueTarget;
    final label = target == null
        ? '${work.progress}%'
        : '${target.unit.displayLabel} · ${work.progress}%';

    return Material(
      color: colors.secondaryContainer,
      borderRadius: BorderRadius.circular(desktop ? 20 : 16),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () async {
          await context.push(work.readerRoute());
          if (mounted) await _load();
        },
        child: Padding(
          padding: EdgeInsets.all(desktop ? 20 : 16),
          child: Row(
            children: [
              SizedBox(
                width: desktop ? 72 : 64,
                height: desktop ? 96 : 88,
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: AuthenticatedImage(
                    imageUrl: work.resolvedCoverUrl(serverUrl),
                    fit: BoxFit.cover,
                    placeholder:
                        ColoredBox(color: colors.surfaceContainerHighest),
                    errorWidget:
                        ColoredBox(color: colors.surfaceContainerHighest),
                  ),
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
                      work.title,
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
                        value: work.progress / 100,
                        minHeight: 4,
                        color: colors.primary,
                        backgroundColor:
                            colors.onSurface.withValues(alpha: 0.12),
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      label,
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
                  onPressed: () async {
                    await context.push(work.readerRoute());
                    if (mounted) await _load();
                  },
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
        ),
      ),
    );
  }
}
