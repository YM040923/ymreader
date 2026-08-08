import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../data/api/comic_api.dart';
import '../../data/models/folder_stats.dart';

typedef FolderStatsLoader = Future<FolderStatsScreenData> Function(
  FolderStatsScope scope,
);

class FolderStatsScreenData {
  final FolderStatsResponse tree;
  final FileStatsSummary summary;

  const FolderStatsScreenData({
    required this.tree,
    required this.summary,
  });
}

class FolderTreeStatsScreen extends ConsumerStatefulWidget {
  final FolderStatsLoader? loadStats;
  final ValueChanged<FolderStatsFile>? onOpenFile;

  const FolderTreeStatsScreen({
    super.key,
    this.loadStats,
    this.onOpenFile,
  });

  @override
  ConsumerState<FolderTreeStatsScreen> createState() =>
      _FolderTreeStatsScreenState();
}

class _FolderTreeStatsScreenState extends ConsumerState<FolderTreeStatsScreen> {
  FolderStatsScope _scope = FolderStatsScope.work;
  bool _loading = true;
  String? _error;
  FolderStatsScreenData? _data;
  int _requestGeneration = 0;

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<FolderStatsScreenData> _loadFromApi(FolderStatsScope scope) async {
    final api = ref.read(comicApiProvider);
    final results = await Future.wait<Object>([
      api.getFolderTreeStats(scope: scope),
      api.getFileStats(scope: scope),
    ]);
    return FolderStatsScreenData(
      tree: results[0] as FolderStatsResponse,
      summary: results[1] as FileStatsSummary,
    );
  }

  Future<void> _loadData() async {
    final generation = ++_requestGeneration;
    setState(() {
      _loading = true;
      _error = null;
      _data = null;
    });
    try {
      final loader = widget.loadStats ?? _loadFromApi;
      final data = await loader(_scope);
      if (!mounted || generation != _requestGeneration) return;
      setState(() {
        _data = data;
        _loading = false;
      });
    } catch (error) {
      if (!mounted || generation != _requestGeneration) return;
      setState(() {
        _loading = false;
        _error = '加载失败：$error';
      });
    }
  }

  void _changeScope(FolderStatsScope scope) {
    if (_scope == scope) return;
    setState(() => _scope = scope);
    _loadData();
  }

  void _openFile(FolderStatsFile file) {
    final callback = widget.onOpenFile;
    if (callback != null) {
      callback(file);
      return;
    }
    final id = Uri.encodeComponent(file.id);
    if (file.type == 'work') {
      context.push('/work/$id');
    } else if (file.type == 'novel') {
      context.push('/novel/$id');
    } else {
      context.push('/comic/$id');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('文件夹统计'),
        actions: [
          IconButton(
            tooltip: '刷新',
            icon: const Icon(Icons.refresh_rounded),
            onPressed: _loading ? null : _loadData,
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: SizedBox(
              width: double.infinity,
              child: SegmentedButton<FolderStatsScope>(
                segments: const [
                  ButtonSegment(
                    value: FolderStatsScope.work,
                    icon: Icon(Icons.menu_book_rounded),
                    label: Text('按作品'),
                  ),
                  ButtonSegment(
                    value: FolderStatsScope.physical,
                    icon: Icon(Icons.insert_drive_file_rounded),
                    label: Text('按物理文件'),
                  ),
                ],
                selected: {_scope},
                showSelectedIcon: false,
                onSelectionChanged: (selection) =>
                    _changeScope(selection.first),
              ),
            ),
          ),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return _MessageState(
        icon: Icons.error_outline_rounded,
        message: _error!,
        actionLabel: '重试',
        onAction: _loadData,
      );
    }
    final data = _data;
    if (data == null || data.tree.roots.isEmpty) {
      return _MessageState(
        icon: Icons.folder_off_outlined,
        message: '暂无统计数据',
        actionLabel: '刷新',
        onAction: _loadData,
      );
    }
    return RefreshIndicator(
      onRefresh: _loadData,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          _SummaryCard(summary: data.summary, scope: _scope),
          const SizedBox(height: 16),
          for (final node in data.tree.roots)
            _FolderTile(
              node: node,
              depth: 0,
              onOpenFile: _openFile,
            ),
        ],
      ),
    );
  }
}

class _SummaryCard extends StatelessWidget {
  final FileStatsSummary summary;
  final FolderStatsScope scope;

  const _SummaryCard({required this.summary, required this.scope});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Card(
      elevation: 0,
      color: colors.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              scope == FolderStatsScope.work ? '作品统计' : '物理文件统计',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 14),
            Row(
              children: [
                Expanded(
                  child: _SummaryValue(
                    label: scope == FolderStatsScope.work ? '作品数' : '文件数',
                    value: '${summary.totalFiles}',
                  ),
                ),
                Expanded(
                  child: _SummaryValue(
                    label: '总页数',
                    value: '${summary.totalPages}',
                  ),
                ),
                Expanded(
                  child: _SummaryValue(
                    label: '总大小',
                    value: _formatSize(summary.totalSize),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SummaryValue extends StatelessWidget {
  final String label;
  final String value;

  const _SummaryValue({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text(
          value,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.w700,
              ),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        const SizedBox(height: 2),
        Text(label, style: Theme.of(context).textTheme.labelSmall),
      ],
    );
  }
}

class _FolderTile extends StatefulWidget {
  final FolderStatsNode node;
  final int depth;
  final ValueChanged<FolderStatsFile> onOpenFile;

  const _FolderTile({
    required this.node,
    required this.depth,
    required this.onOpenFile,
  });

  @override
  State<_FolderTile> createState() => _FolderTileState();
}

class _FolderTileState extends State<_FolderTile> {
  late bool _expanded;

  @override
  void initState() {
    super.initState();
    _expanded = widget.depth == 0;
  }

  @override
  Widget build(BuildContext context) {
    final node = widget.node;
    final hasContent = node.children.isNotEmpty || node.files.isNotEmpty;
    final colors = Theme.of(context).colorScheme;
    return Padding(
      padding: EdgeInsets.only(left: widget.depth * 12.0, bottom: 6),
      child: Column(
        children: [
          Material(
            color: colors.surfaceContainerLow,
            borderRadius: BorderRadius.circular(12),
            child: ListTile(
              dense: true,
              onTap: hasContent
                  ? () => setState(() => _expanded = !_expanded)
                  : null,
              leading: Icon(
                _expanded ? Icons.folder_open_rounded : Icons.folder_rounded,
                color: colors.primary,
              ),
              title: Text(
                node.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              subtitle: Text(
                '${node.fileCount} 项 · ${node.totalPages} 页 · '
                '${_formatSize(node.totalSize)}',
              ),
              trailing: hasContent
                  ? Icon(
                      _expanded
                          ? Icons.expand_less_rounded
                          : Icons.expand_more_rounded,
                    )
                  : null,
            ),
          ),
          if (_expanded) ...[
            for (final child in node.children)
              _FolderTile(
                node: child,
                depth: widget.depth + 1,
                onOpenFile: widget.onOpenFile,
              ),
            for (final file in node.files)
              Padding(
                padding: EdgeInsets.only(
                  left: (widget.depth + 1) * 12.0,
                  top: 4,
                ),
                child: _FolderFileTile(
                  file: file,
                  onTap: () => widget.onOpenFile(file),
                ),
              ),
          ],
        ],
      ),
    );
  }
}

class _FolderFileTile extends StatelessWidget {
  final FolderStatsFile file;
  final VoidCallback onTap;

  const _FolderFileTile({required this.file, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final isWork = file.type == 'work';
    return ListTile(
      dense: true,
      contentPadding: const EdgeInsets.symmetric(horizontal: 12),
      leading: Icon(
        isWork ? Icons.menu_book_rounded : Icons.insert_drive_file_outlined,
      ),
      title: Text(
        file.title.isNotEmpty ? file.title : file.filename,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text('${file.pageCount} 页 · ${_formatSize(file.fileSize)}'),
      trailing: const Icon(Icons.chevron_right_rounded),
      onTap: onTap,
    );
  }
}

class _MessageState extends StatelessWidget {
  final IconData icon;
  final String message;
  final String actionLabel;
  final VoidCallback onAction;

  const _MessageState({
    required this.icon,
    required this.message,
    required this.actionLabel,
    required this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 42),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 12),
            FilledButton.tonal(onPressed: onAction, child: Text(actionLabel)),
          ],
        ),
      ),
    );
  }
}

String _formatSize(int bytes) {
  if (bytes < 1024) return '$bytes B';
  if (bytes < 1024 * 1024) {
    return '${(bytes / 1024).toStringAsFixed(1)} KB';
  }
  if (bytes < 1024 * 1024 * 1024) {
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }
  return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB';
}
