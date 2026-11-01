import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../widgets/authenticated_image.dart';

class MergeWorkDialog extends ConsumerStatefulWidget {
  final Work work;

  const MergeWorkDialog({super.key, required this.work});

  @override
  ConsumerState<MergeWorkDialog> createState() => _MergeWorkDialogState();
}

class _MergeWorkDialogState extends ConsumerState<MergeWorkDialog> {
  final TextEditingController _search = TextEditingController();
  List<Work> _works = const [];
  Work? _target;
  bool _loading = true;
  bool _merging = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final collected = <Work>[];
      var page = 1;
      var totalPages = 1;
      while (page <= totalPages && page <= 50) {
        final response = await ref.read(workApiProvider).listWorks(
              WorkQuery(
                page: page,
                pageSize: 200,
                libraryIds: [widget.work.libraryId],
              ),
            );
        collected.addAll(response.works);
        totalPages = response.totalPages;
        if (response.works.isEmpty) break;
        page += 1;
      }
      final unique = <String, Work>{};
      for (final work in collected) {
        unique[work.id] = work;
      }
      if (!mounted) return;
      setState(() {
        _works = unique.values
            .where((candidate) =>
                candidate.id != widget.work.id &&
                (widget.work.libraryId.isEmpty ||
                    candidate.libraryId == widget.work.libraryId))
            .toList();
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = '$error';
        _loading = false;
      });
    }
  }

  Future<void> _confirm() async {
    final target = _target;
    if (target == null || _merging) return;
    setState(() {
      _merging = true;
      _error = null;
    });
    try {
      await ref.read(workApiProvider).mergeWorks([target.id, widget.work.id]);
      if (mounted) Navigator.pop(context, target.id);
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = '$error';
          _merging = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final serverUrl = ref.watch(authProvider).serverUrl;
    final filtered = _works.where((candidate) {
      final query = _search.text.trim().toLowerCase();
      return query.isEmpty || candidate.title.toLowerCase().contains(query);
    }).toList();
    return Dialog(
      insetPadding: const EdgeInsets.all(18),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.sizeOf(context).height * 0.78,
        ),
        child: Padding(
          padding: const EdgeInsets.all(18),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      '合并作品',
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                  ),
                  IconButton(
                    tooltip: '关闭',
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
              Text(
                '把当前作品并入所选作品，章节、阅读进度和元数据会保留。',
                style: TextStyle(color: colors.onSurfaceVariant),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _search,
                onChanged: (_) => setState(() {}),
                decoration: const InputDecoration(
                  prefixIcon: Icon(Icons.search_rounded),
                  hintText: '搜索目标作品',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 8),
                Text(_error!, style: TextStyle(color: colors.error)),
              ],
              const SizedBox(height: 10),
              Flexible(
                child: _loading
                    ? const Center(child: CircularProgressIndicator())
                    : filtered.isEmpty
                        ? const Center(child: Text('没有可合并的作品'))
                        : ListView.builder(
                            shrinkWrap: true,
                            itemCount: filtered.length,
                            itemBuilder: (context, index) {
                              final candidate = filtered[index];
                              final selected = candidate.id == _target?.id;
                              return ListTile(
                                selected: selected,
                                contentPadding: EdgeInsets.zero,
                                leading: ClipRRect(
                                  borderRadius: BorderRadius.circular(6),
                                  child: SizedBox(
                                    width: 36,
                                    height: 50,
                                    child: AuthenticatedImage(
                                      imageUrl:
                                          candidate.resolvedCoverUrl(serverUrl),
                                      fit: BoxFit.cover,
                                      placeholder: ColoredBox(
                                        color: colors.surfaceContainerHighest,
                                      ),
                                      errorWidget: ColoredBox(
                                        color: colors.surfaceContainerHighest,
                                      ),
                                    ),
                                  ),
                                ),
                                title: Text(
                                  candidate.title,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                subtitle: Text(
                                  '${candidate.itemCount} 话/卷 · ${candidate.pageCount} 页',
                                ),
                                trailing: selected
                                    ? Icon(
                                        Icons.check_circle_rounded,
                                        color: colors.primary,
                                      )
                                    : null,
                                onTap: () =>
                                    setState(() => _target = candidate),
                              );
                            },
                          ),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      _target == null ? '选择目标作品' : '并入《${_target!.title}》',
                      style: TextStyle(color: colors.onSurfaceVariant),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  FilledButton(
                    onPressed: _target == null || _merging ? null : _confirm,
                    child: Text(_merging ? '合并中…' : '确认合并'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class SplitWorkSheet extends ConsumerStatefulWidget {
  final Work work;

  const SplitWorkSheet({super.key, required this.work});

  @override
  ConsumerState<SplitWorkSheet> createState() => _SplitWorkSheetState();
}

class _SplitWorkSheetState extends ConsumerState<SplitWorkSheet> {
  final Set<String> _selected = <String>{};
  final TextEditingController _title = TextEditingController();
  bool _splitting = false;
  String? _error;

  @override
  void dispose() {
    _title.dispose();
    super.dispose();
  }

  bool get _hasPartialArchive {
    final totalByComic = <String, int>{};
    final selectedByComic = <String, int>{};
    for (final unit in widget.work.units) {
      totalByComic[unit.comicId] = (totalByComic[unit.comicId] ?? 0) + 1;
      if (_selected.contains(unit.id)) {
        selectedByComic[unit.comicId] =
            (selectedByComic[unit.comicId] ?? 0) + 1;
      }
    }
    return selectedByComic.entries.any((entry) {
      final total = totalByComic[entry.key] ?? 0;
      return entry.value > 0 && entry.value < total;
    });
  }

  Future<void> _confirm() async {
    if (_selected.isEmpty ||
        _selected.length >= widget.work.units.length ||
        _hasPartialArchive ||
        _splitting) {
      return;
    }
    setState(() {
      _splitting = true;
      _error = null;
    });
    try {
      final newWorkId = await ref.read(workApiProvider).splitWork(
            widget.work.id,
            _selected.toList(),
            title: _title.text,
          );
      if (newWorkId.isEmpty) {
        throw StateError('服务器未返回新作品 ID');
      }
      if (mounted) {
        Navigator.pop(context, newWorkId);
      }
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = '$error';
          _splitting = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final allSelected = _selected.length >= widget.work.units.length;
    final partialArchive = _hasPartialArchive;
    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.viewInsetsOf(context).bottom,
      ),
      child: SafeArea(
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * 0.82,
          ),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(18, 16, 18, 18),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '拆分单元',
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                    ),
                    IconButton(
                      tooltip: '关闭',
                      onPressed: () => Navigator.pop(context),
                      icon: const Icon(Icons.close_rounded),
                    ),
                  ],
                ),
                Text(
                  '选择要移入新作品的单元；原作品至少保留一个单元。',
                  style: TextStyle(color: colors.onSurfaceVariant),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 6),
                  Text(_error!, style: TextStyle(color: colors.error)),
                ],
                const SizedBox(height: 8),
                Flexible(
                  child: ListView.builder(
                    shrinkWrap: true,
                    itemCount: widget.work.units.length,
                    itemBuilder: (context, index) {
                      final unit = widget.work.units[index];
                      final selected = _selected.contains(unit.id);
                      return CheckboxListTile(
                        value: selected,
                        contentPadding: EdgeInsets.zero,
                        title: Text(
                          unit.displayLabel.isEmpty
                              ? unit.title
                              : unit.displayLabel,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        subtitle: Text('${unit.pageCount} 页'),
                        onChanged: (value) {
                          setState(() {
                            if (value == true) {
                              _selected.add(unit.id);
                            } else {
                              _selected.remove(unit.id);
                            }
                          });
                        },
                      );
                    },
                  ),
                ),
                if (allSelected)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Text(
                      '至少保留一个单元',
                      style: TextStyle(color: colors.tertiary),
                    ),
                  ),
                if (partialArchive)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Text(
                      '同一压缩包内的单元不能只拆出一部分',
                      style: TextStyle(color: colors.error),
                    ),
                  ),
                TextField(
                  controller: _title,
                  decoration: const InputDecoration(
                    labelText: '新作品标题（可选）',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '已选 ${_selected.length} 个单元',
                        style: TextStyle(color: colors.onSurfaceVariant),
                      ),
                    ),
                    OutlinedButton(
                      onPressed:
                          _splitting ? null : () => Navigator.pop(context),
                      child: const Text('取消'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: _selected.isEmpty ||
                              allSelected ||
                              partialArchive ||
                              _splitting
                          ? null
                          : _confirm,
                      child: Text(_splitting ? '拆分中…' : '确认拆分'),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
