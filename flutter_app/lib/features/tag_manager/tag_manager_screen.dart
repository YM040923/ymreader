import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/models/comic.dart';
import '../../data/providers/auth_provider.dart';
import '../../data/providers/tag_manager_provider.dart';

/// 标签与分类管理页面
class TagManagerScreen extends ConsumerStatefulWidget {
  const TagManagerScreen({super.key});

  @override
  ConsumerState<TagManagerScreen> createState() => _TagManagerScreenState();
}

class _TagManagerScreenState extends ConsumerState<TagManagerScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  String _tagSearch = '';
  String _catSearch = '';

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(tagManagerProvider);
    final authState = ref.watch(authProvider);
    final isAdmin = authState.user?.isAdmin ?? false;
    final cs = Theme.of(context).colorScheme;

    // 监听错误并显示 SnackBar
    ref.listen<TagManagerState>(tagManagerProvider, (prev, next) {
      if (next.error != null && next.error != prev?.error) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(next.error!),
            backgroundColor: cs.error,
            behavior: SnackBarBehavior.floating,
            action: SnackBarAction(
              label: '关闭',
              textColor: cs.onError,
              onPressed: () => ref.read(tagManagerProvider.notifier).clearError(),
            ),
          ),
        );
      }
    });

    return Scaffold(
      appBar: AppBar(
        title: const Text('标签与分类管理'),
        bottom: TabBar(
          controller: _tabController,
          tabs: [
            Tab(
              icon: const Icon(Icons.label_outlined),
              text: '标签 (${state.tags.length})',
            ),
            Tab(
              icon: const Icon(Icons.category_outlined),
              text: '分类 (${state.categories.length})',
            ),
          ],
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
            onPressed: () => ref.read(tagManagerProvider.notifier).loadAll(),
          ),
        ],
      ),
      body: state.isLoading
          ? const Center(child: CircularProgressIndicator())
          : TabBarView(
              controller: _tabController,
              children: [
                _TagsTab(
                  tags: state.tags,
                  selectedIds: state.selectedTagIds,
                  searchQuery: _tagSearch,
                  isAdmin: isAdmin,
                  onSearchChanged: (v) => setState(() => _tagSearch = v),
                ),
                _CategoriesTab(
                  categories: state.categories,
                  selectedIds: state.selectedCategoryIds,
                  searchQuery: _catSearch,
                  isAdmin: isAdmin,
                  onSearchChanged: (v) => setState(() => _catSearch = v),
                ),
              ],
            ),
    );
  }
}

// ============================================================
// 标签管理 Tab
// ============================================================
class _TagsTab extends ConsumerWidget {
  final List<Tag> tags;
  final Set<int> selectedIds;
  final String searchQuery;
  final bool isAdmin;
  final ValueChanged<String> onSearchChanged;

  const _TagsTab({
    required this.tags,
    required this.selectedIds,
    required this.searchQuery,
    required this.isAdmin,
    required this.onSearchChanged,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cs = Theme.of(context).colorScheme;
    final filtered = tags.where((t) {
      if (searchQuery.isEmpty) return true;
      return t.name.toLowerCase().contains(searchQuery.toLowerCase());
    }).toList();

    return Column(
      children: [
        // 工具栏
        if (isAdmin)
          _buildToolbar(context, ref, cs),
        // 搜索栏
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
          child: TextField(
            decoration: InputDecoration(
              hintText: '搜索标签...',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: searchQuery.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: () => onSearchChanged(''),
                    )
                  : null,
            ),
            onChanged: onSearchChanged,
          ),
        ),
        // 列表
        Expanded(
          child: filtered.isEmpty
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.label_off, size: 48, color: cs.onSurfaceVariant),
                      const SizedBox(height: 8),
                      Text(
                        searchQuery.isEmpty ? '暂无标签' : '未找到匹配的标签',
                        style: Theme.of(context).textTheme.bodyLarge,
                      ),
                    ],
                  ),
                )
              : ListView.builder(
                  itemCount: filtered.length,
                  padding: const EdgeInsets.only(bottom: 80),
                  itemBuilder: (context, index) {
                    final tag = filtered[index];
                    final isSelected = selectedIds.contains(tag.id);

                    return ListTile(
                      leading: isAdmin
                          ? Checkbox(
                              value: isSelected,
                              onChanged: (_) => ref
                                  .read(tagManagerProvider.notifier)
                                  .toggleTagSelection(tag.id),
                            )
                          : _buildColorDot(tag.color, cs),
                      title: Text(tag.name),
                      subtitle: tag.color.isNotEmpty
                          ? Row(
                              children: [
                                Container(
                                  width: 12,
                                  height: 12,
                                  decoration: BoxDecoration(
                                    color: _parseColor(tag.color, cs.primary),
                                    shape: BoxShape.circle,
                                  ),
                                ),
                                const SizedBox(width: 6),
                                Text(tag.color),
                              ],
                            )
                          : null,
                      trailing: isAdmin
                          ? PopupMenuButton<String>(
                              itemBuilder: (_) => [
                                const PopupMenuItem(
                                  value: 'rename',
                                  child: ListTile(
                                    leading: Icon(Icons.edit),
                                    title: Text('重命名'),
                                    dense: true,
                                  ),
                                ),
                                const PopupMenuItem(
                                  value: 'color',
                                  child: ListTile(
                                    leading: Icon(Icons.palette),
                                    title: Text('修改颜色'),
                                    dense: true,
                                  ),
                                ),
                                const PopupMenuItem(
                                  value: 'delete',
                                  child: ListTile(
                                    leading: Icon(Icons.delete, color: Colors.red),
                                    title: Text('删除', style: TextStyle(color: Colors.red)),
                                    dense: true,
                                  ),
                                ),
                              ],
                              onSelected: (action) =>
                                  _handleTagAction(context, ref, tag, action),
                            )
                          : null,
                    );
                  },
                ),
        ),
      ],
    );
  }

  Widget _buildToolbar(BuildContext context, WidgetRef ref, ColorScheme cs) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: cs.surfaceContainerLow,
        border: Border(bottom: BorderSide(color: cs.outlineVariant, width: 0.5)),
      ),
      child: Row(
        children: [
          // 全选/取消全选
          TextButton.icon(
            icon: Icon(
              selectedIds.length == tags.length && tags.isNotEmpty
                  ? Icons.check_box
                  : Icons.check_box_outline_blank,
              size: 20,
            ),
            label: Text(selectedIds.isEmpty ? '全选' : '${selectedIds.length} 已选'),
            onPressed: () {
              if (selectedIds.length == tags.length) {
                ref.read(tagManagerProvider.notifier).clearTagSelection();
              } else {
                ref.read(tagManagerProvider.notifier).selectAllTags();
              }
            },
          ),
          const Spacer(),
          if (selectedIds.isNotEmpty) ...[
            // 合并
            if (selectedIds.length >= 2)
              TextButton.icon(
                icon: const Icon(Icons.merge_type, size: 20),
                label: const Text('合并'),
                onPressed: () => _showMergeDialog(context, ref),
              ),
            const SizedBox(width: 8),
            // 批量删除
            TextButton.icon(
              icon: Icon(Icons.delete, size: 20, color: cs.error),
              label: Text('删除', style: TextStyle(color: cs.error)),
              onPressed: () => _confirmDeleteSelected(context, ref),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildColorDot(String color, ColorScheme cs) {
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(
        color: color.isNotEmpty ? _parseColor(color, cs.primary) : cs.primaryContainer,
        shape: BoxShape.circle,
      ),
      child: Icon(Icons.label, size: 16, color: cs.onPrimaryContainer),
    );
  }

  void _handleTagAction(
      BuildContext context, WidgetRef ref, Tag tag, String action) {
    switch (action) {
      case 'rename':
        _showRenameDialog(context, ref, tag);
      case 'color':
        _showColorDialog(context, ref, tag);
      case 'delete':
        _confirmDeleteTag(context, ref, tag);
    }
  }

  void _showRenameDialog(BuildContext context, WidgetRef ref, Tag tag) {
    final controller = TextEditingController(text: tag.name);
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('重命名标签'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '标签名称',
            hintText: '输入新名称',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              final name = controller.text.trim();
              if (name.isNotEmpty && name != tag.name) {
                await ref.read(tagManagerProvider.notifier).renameTag(tag.id, name);
              }
              if (ctx.mounted) Navigator.pop(ctx);
            },
            child: const Text('确定'),
          ),
        ],
      ),
    );
  }

  void _showColorDialog(BuildContext context, WidgetRef ref, Tag tag) {
    final colors = [
      '#EF4444', '#F97316', '#EAB308', '#22C55E', '#06B6D4',
      '#3B82F6', '#6366F1', '#8B5CF6', '#EC4899', '#F43F5E',
      '#14B8A6', '#84CC16', '#A855F7', '#78716C',
    ];
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('选择颜色'),
        content: Wrap(
          spacing: 8,
          runSpacing: 8,
          children: colors.map((c) {
            return InkWell(
              onTap: () async {
                await ref.read(tagManagerProvider.notifier).updateTagColor(tag.id, c);
                if (ctx.mounted) Navigator.pop(ctx);
              },
              borderRadius: BorderRadius.circular(20),
              child: Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: _parseColor(c, Colors.grey),
                  shape: BoxShape.circle,
                  border: tag.color == c
                      ? Border.all(color: Colors.white, width: 3)
                      : null,
                  boxShadow: tag.color == c
                      ? [BoxShadow(color: _parseColor(c, Colors.grey).withValues(alpha: 0.5), blurRadius: 8)]
                      : null,
                ),
              ),
            );
          }).toList(),
        ),
      ),
    );
  }

  void _confirmDeleteTag(BuildContext context, WidgetRef ref, Tag tag) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除标签'),
        content: Text('确定要删除标签「${tag.name}」吗？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              await ref.read(tagManagerProvider.notifier).deleteTag(tag.id);
              if (ctx.mounted) Navigator.pop(ctx);
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('删除'),
          ),
        ],
      ),
    );
  }

  void _confirmDeleteSelected(BuildContext context, WidgetRef ref) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('批量删除'),
        content: Text('确定要删除选中的 ${selectedIds.length} 个标签吗？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              await ref.read(tagManagerProvider.notifier).deleteSelectedTags();
              if (ctx.mounted) Navigator.pop(ctx);
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('删除'),
          ),
        ],
      ),
    );
  }

  void _showMergeDialog(BuildContext context, WidgetRef ref) {
    final selectedTags = tags.where((t) => selectedIds.contains(t.id)).toList();
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('合并标签'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('选择要保留的目标标签（其他标签将合并到该标签）：'),
            const SizedBox(height: 12),
            ...selectedTags.map((t) => ListTile(
                  title: Text(t.name),
                  leading: const Icon(Icons.label),
                  onTap: () async {
                    Navigator.pop(ctx);
                    await ref.read(tagManagerProvider.notifier).mergeTags(t.id);
                  },
                )),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
        ],
      ),
    );
  }
}

// ============================================================
// 分类管理 Tab
// ============================================================
class _CategoriesTab extends ConsumerWidget {
  final List<Category> categories;
  final Set<int> selectedIds;
  final String searchQuery;
  final bool isAdmin;
  final ValueChanged<String> onSearchChanged;

  const _CategoriesTab({
    required this.categories,
    required this.selectedIds,
    required this.searchQuery,
    required this.isAdmin,
    required this.onSearchChanged,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cs = Theme.of(context).colorScheme;
    final filtered = categories.where((c) {
      if (searchQuery.isEmpty) return true;
      return c.name.toLowerCase().contains(searchQuery.toLowerCase()) ||
          c.slug.toLowerCase().contains(searchQuery.toLowerCase());
    }).toList();

    return Column(
      children: [
        // 工具栏
        if (isAdmin) _buildToolbar(context, ref, cs),
        // 搜索栏
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
          child: TextField(
            decoration: InputDecoration(
              hintText: '搜索分类...',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: searchQuery.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: () => onSearchChanged(''),
                    )
                  : null,
            ),
            onChanged: onSearchChanged,
          ),
        ),
        // 列表
        Expanded(
          child: filtered.isEmpty
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.category_outlined, size: 48, color: cs.onSurfaceVariant),
                      const SizedBox(height: 8),
                      Text(
                        searchQuery.isEmpty ? '暂无分类' : '未找到匹配的分类',
                        style: Theme.of(context).textTheme.bodyLarge,
                      ),
                    ],
                  ),
                )
              : ListView.builder(
                  itemCount: filtered.length,
                  padding: const EdgeInsets.only(bottom: 80),
                  itemBuilder: (context, index) {
                    final cat = filtered[index];
                    final isSelected = selectedIds.contains(cat.id);

                    return ListTile(
                      leading: isAdmin
                          ? Checkbox(
                              value: isSelected,
                              onChanged: (_) => ref
                                  .read(tagManagerProvider.notifier)
                                  .toggleCategorySelection(cat.id),
                            )
                          : CircleAvatar(
                              backgroundColor: cs.secondaryContainer,
                              child: Icon(Icons.folder, color: cs.onSecondaryContainer),
                            ),
                      title: Text(cat.name),
                      subtitle: Text(cat.slug, style: TextStyle(color: cs.onSurfaceVariant)),
                      trailing: isAdmin
                          ? PopupMenuButton<String>(
                              itemBuilder: (_) => [
                                const PopupMenuItem(
                                  value: 'edit',
                                  child: ListTile(
                                    leading: Icon(Icons.edit),
                                    title: Text('编辑'),
                                    dense: true,
                                  ),
                                ),
                                const PopupMenuItem(
                                  value: 'delete',
                                  child: ListTile(
                                    leading: Icon(Icons.delete, color: Colors.red),
                                    title: Text('删除', style: TextStyle(color: Colors.red)),
                                    dense: true,
                                  ),
                                ),
                              ],
                              onSelected: (action) =>
                                  _handleCategoryAction(context, ref, cat, action),
                            )
                          : null,
                    );
                  },
                ),
        ),
      ],
    );
  }

  Widget _buildToolbar(BuildContext context, WidgetRef ref, ColorScheme cs) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: cs.surfaceContainerLow,
        border: Border(bottom: BorderSide(color: cs.outlineVariant, width: 0.5)),
      ),
      child: Row(
        children: [
          // 全选/取消全选
          TextButton.icon(
            icon: Icon(
              selectedIds.length == categories.length && categories.isNotEmpty
                  ? Icons.check_box
                  : Icons.check_box_outline_blank,
              size: 20,
            ),
            label: Text(selectedIds.isEmpty ? '全选' : '${selectedIds.length} 已选'),
            onPressed: () {
              if (selectedIds.length == categories.length) {
                ref.read(tagManagerProvider.notifier).clearCategorySelection();
              } else {
                ref.read(tagManagerProvider.notifier).selectAllCategories();
              }
            },
          ),
          const Spacer(),
          // 新建分类
          TextButton.icon(
            icon: const Icon(Icons.add, size: 20),
            label: const Text('新建'),
            onPressed: () => _showCreateDialog(context, ref),
          ),
          if (selectedIds.isNotEmpty) ...[
            const SizedBox(width: 8),
            TextButton.icon(
              icon: Icon(Icons.delete, size: 20, color: cs.error),
              label: Text('删除', style: TextStyle(color: cs.error)),
              onPressed: () => _confirmDeleteSelected(context, ref),
            ),
          ],
        ],
      ),
    );
  }

  void _handleCategoryAction(
      BuildContext context, WidgetRef ref, Category cat, String action) {
    switch (action) {
      case 'edit':
        _showEditDialog(context, ref, cat);
      case 'delete':
        _confirmDeleteCategory(context, ref, cat);
    }
  }

  void _showCreateDialog(BuildContext context, WidgetRef ref) {
    final controller = TextEditingController();
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('新建分类'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '分类名称',
            hintText: '输入分类名称',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              final name = controller.text.trim();
              if (name.isNotEmpty) {
                await ref.read(tagManagerProvider.notifier).createCategory(name);
              }
              if (ctx.mounted) Navigator.pop(ctx);
            },
            child: const Text('创建'),
          ),
        ],
      ),
    );
  }

  void _showEditDialog(BuildContext context, WidgetRef ref, Category cat) {
    final nameController = TextEditingController(text: cat.name);
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('编辑分类'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: nameController,
              autofocus: true,
              decoration: const InputDecoration(
                labelText: '分类名称',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              final name = nameController.text.trim();
              if (name.isNotEmpty && name != cat.name) {
                await ref
                    .read(tagManagerProvider.notifier)
                    .updateCategory(cat.slug, name: name);
              }
              if (ctx.mounted) Navigator.pop(ctx);
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
  }

  void _confirmDeleteCategory(BuildContext context, WidgetRef ref, Category cat) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除分类'),
        content: Text('确定要删除分类「${cat.name}」吗？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              await ref.read(tagManagerProvider.notifier).deleteCategory(cat.slug);
              if (ctx.mounted) Navigator.pop(ctx);
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('删除'),
          ),
        ],
      ),
    );
  }

  void _confirmDeleteSelected(BuildContext context, WidgetRef ref) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('批量删除'),
        content: Text('确定要删除选中的 ${selectedIds.length} 个分类吗？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () async {
              await ref
                  .read(tagManagerProvider.notifier)
                  .deleteSelectedCategories();
              if (ctx.mounted) Navigator.pop(ctx);
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('删除'),
          ),
        ],
      ),
    );
  }
}

// ============================================================
// 工具函数
// ============================================================

/// 将十六进制颜色字符串转换为 Color 对象
Color _parseColor(String hex, Color fallback) {
  try {
    if (hex.isEmpty) return fallback;
    hex = hex.replaceFirst('#', '');
    if (hex.length == 6) hex = 'FF$hex';
    return Color(int.parse(hex, radix: 16));
  } catch (_) {
    return fallback;
  }
}
