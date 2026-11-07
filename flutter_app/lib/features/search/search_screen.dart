import 'package:flutter/material.dart';
import '../../widgets/phosphor_icons.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../widgets/responsive.dart';
import '../../widgets/work_card.dart';
import '../shell/app_shell.dart';

class SearchScreen extends ConsumerStatefulWidget {
  final bool discover;

  const SearchScreen({super.key, this.discover = false});

  @override
  ConsumerState<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends ConsumerState<SearchScreen> {
  final _controller = TextEditingController();
  List<Work> _results = const [];
  List<WorkTagStat> _tags = const [];
  List<WorkCategoryStat> _categories = const [];
  String? _tag;
  String? _category;
  bool _loading = false;
  bool _searched = false;

  @override
  void initState() {
    super.initState();
    _loadFilters();
    if (widget.discover) _search();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _loadFilters() async {
    try {
      final api = ref.read(workApiProvider);
      final values =
          await Future.wait([api.getTagStats(), api.getCategoryStats()]);
      if (!mounted) return;
      setState(() {
        _tags = values[0] as List<WorkTagStat>;
        _categories = values[1] as List<WorkCategoryStat>;
      });
    } catch (_) {}
  }

  Future<void> _openTagSheet() async {
    final picked = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      useRootNavigator: true,
      useSafeArea: true,
      builder: (_) => _TagSheet(tags: _tags, selected: _tag),
    );
    if (picked == null) return;
    setState(() => _tag = picked.isEmpty ? null : picked);
    _search();
  }

  Future<void> _search() async {
    setState(() {
      _loading = true;
      _searched = true;
    });
    try {
      final response = await ref.read(workApiProvider).listWorks(
            WorkQuery(
              pageSize: 100,
              search: _controller.text.trim(),
              tags: _tag == null ? const [] : [_tag!],
              category: _category,
              sortBy: widget.discover ? 'addedAt' : 'title',
              sortOrder: widget.discover ? 'desc' : 'asc',
            ),
          );
      if (!mounted) return;
      setState(() {
        _results = response.works;
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final serverUrl = ref.watch(authProvider).serverUrl;
    final width = MediaQuery.sizeOf(context).width;
    final columns = coverColumns(width);
    // 标题、搜索胶囊、分类和封面网格共用同一条页面边线；手机上仍是 16。
    final titlePadding = pagePadding(width);
    final gridHorizontal = pagePadding(width, phone: 12);
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.discover ? '发现' : '搜索'),
        titleSpacing: titlePadding,
      ),
      body: Column(
        children: [
          Padding(
            padding: EdgeInsets.fromLTRB(titlePadding, 6, titlePadding, 8),
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: widget.discover ? () => context.push('/search') : null,
              child: Container(
                height: 36,
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(18),
                ),
                child: Row(
                  children: [
                    const SizedBox(width: 12),
                    Icon(
                      PhosphorIconsRegular.magnifyingGlass,
                      size: 18,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: widget.discover
                          ? Text(
                              '搜索漫画、作者或目录名称',
                              style: TextStyle(
                                fontSize: 14,
                                color: Theme.of(context).colorScheme.tertiary,
                              ),
                            )
                          : TextField(
                              controller: _controller,
                              textInputAction: TextInputAction.search,
                              onSubmitted: (_) => _search(),
                              style: const TextStyle(fontSize: 14),
                              decoration: InputDecoration(
                                isDense: true,
                                border: InputBorder.none,
                                enabledBorder: InputBorder.none,
                                focusedBorder: InputBorder.none,
                                hintText: '搜索漫画、作者或目录名称',
                                hintStyle: TextStyle(
                                  fontSize: 14,
                                  color: Theme.of(context).colorScheme.tertiary,
                                ),
                                contentPadding: EdgeInsets.zero,
                              ),
                            ),
                    ),
                    if (!widget.discover)
                      SizedBox(
                        width: 36,
                        height: 36,
                        child: IconButton(
                          padding: EdgeInsets.zero,
                          visualDensity: VisualDensity.compact,
                          onPressed: _search,
                          icon: const Icon(PhosphorIconsRegular.arrowRight,
                              size: 19),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: titlePadding),
            child: SizedBox(
              height: 40,
              child: Row(
                children: [
                  Expanded(
                    child: ListView(
                      scrollDirection: Axis.horizontal,
                      padding: const EdgeInsets.only(left: 6, right: 4),
                      children: [
                        _CategoryTab(
                          label: '全部',
                          selected: _category == null,
                          onTap: () {
                            setState(() => _category = null);
                            _search();
                          },
                        ),
                        for (final item in _categories)
                          _CategoryTab(
                            label: item.name,
                            selected: _category == item.slug,
                            onTap: () {
                              setState(() => _category =
                                  _category == item.slug ? null : item.slug);
                              _search();
                            },
                          ),
                      ],
                    ),
                  ),
                  if (_tags.isNotEmpty)
                    _FilterButton(
                      active: _tag != null,
                      label: _tag,
                      onTap: _openTagSheet,
                    ),
                ],
              ),
            ),
          ),
          Expanded(
            child: Stack(
              children: [
                Positioned.fill(
                  child: _results.isNotEmpty
                      ? GridView.builder(
                          padding: EdgeInsets.fromLTRB(
                            gridHorizontal,
                            12,
                            gridHorizontal,
                            12 + shellBarClearance(context),
                          ),
                          gridDelegate:
                              SliverGridDelegateWithFixedCrossAxisCount(
                            crossAxisCount: columns,
                            childAspectRatio: 0.66,
                            crossAxisSpacing: width >= 860 ? 16 : 8,
                            mainAxisSpacing: width >= 860 ? 24 : 16,
                          ),
                          itemCount: _results.length,
                          itemBuilder: (_, index) => WorkCard(
                            work: _results[index],
                            serverUrl: serverUrl,
                          ),
                        )
                      : _loading
                          ? const Center(child: CircularProgressIndicator())
                          : Center(
                              child: Text(
                                _searched ? '没有找到匹配的漫画' : '输入关键词开始搜索',
                              ),
                            ),
                ),
                if (_loading && _results.isNotEmpty)
                  const Positioned(
                    top: 0,
                    left: 0,
                    right: 0,
                    child: LinearProgressIndicator(minHeight: 2),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _CategoryTab extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _CategoryTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10),
        child: Center(
          child: AnimatedDefaultTextStyle(
            duration: const Duration(milliseconds: 160),
            curve: Curves.easeOut,
            style: TextStyle(
              fontSize: selected ? 16 : 14,
              fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              color: selected ? colors.primary : colors.onSurfaceVariant,
              height: 1.2,
            ),
            child: Text(label),
          ),
        ),
      ),
    );
  }
}

class _FilterButton extends StatelessWidget {
  final bool active;
  final String? label;
  final VoidCallback onTap;

  const _FilterButton({
    required this.active,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final accent = colors.primary;
    final fg = active ? accent : colors.onSurfaceVariant;
    return Padding(
      padding: const EdgeInsets.only(left: 8, right: 16),
      child: Material(
        color: active
            ? accent.withValues(alpha: 0.10)
            : colors.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(15),
        child: InkWell(
          borderRadius: BorderRadius.circular(15),
          onTap: onTap,
          child: Container(
            height: 30,
            padding: const EdgeInsets.symmetric(horizontal: 11),
            constraints: const BoxConstraints(maxWidth: 118),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  active
                      ? PhosphorIconsRegular.check
                      : PhosphorIconsRegular.slidersHorizontal,
                  size: 15,
                  color: fg,
                ),
                const SizedBox(width: 5),
                Flexible(
                  child: Text(
                    active ? label! : '筛选',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: fg,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _TagSheet extends StatelessWidget {
  final List<WorkTagStat> tags;
  final String? selected;

  const _TagSheet({required this.tags, required this.selected});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final sorted = [...tags]..sort((a, b) => b.count.compareTo(a.count));
    return Container(
      constraints: BoxConstraints(
        maxHeight: MediaQuery.sizeOf(context).height * 0.62,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Container(
              width: 32,
              height: 4,
              decoration: BoxDecoration(
                color: colors.onSurface.withValues(alpha: 0.18),
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 14, 10, 2),
            child: Row(
              children: [
                Text(
                  '标签筛选',
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: colors.onSurface,
                  ),
                ),
                const Spacer(),
                if (selected != null)
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(''),
                    style: TextButton.styleFrom(
                      foregroundColor: colors.primary,
                      minimumSize: const Size(44, 32),
                      padding: const EdgeInsets.symmetric(horizontal: 10),
                      visualDensity: VisualDensity.compact,
                    ),
                    child: const Text(
                      '清除',
                      style:
                          TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
                    ),
                  ),
              ],
            ),
          ),
          Flexible(
            child: SizedBox(
              width: double.infinity,
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(20, 10, 20, 20),
                child: Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    _TagPill(
                      label: '全部',
                      selected: selected == null,
                      onTap: () => Navigator.of(context).pop(''),
                    ),
                    for (final tag in sorted)
                      _TagPill(
                        label: tag.name,
                        selected: selected == tag.name,
                        onTap: () => Navigator.of(context).pop(tag.name),
                      ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _TagPill extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _TagPill({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Material(
      color: selected
          ? colors.primary.withValues(alpha: 0.10)
          : colors.surfaceContainerHighest,
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: onTap,
        child: Container(
          height: 30,
          padding: const EdgeInsets.symmetric(horizontal: 13),
          child: Center(
            widthFactor: 1,
            heightFactor: 1,
            child: Text(
              label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
                color: selected ? colors.primary : colors.onSurfaceVariant,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
