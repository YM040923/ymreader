import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../widgets/work_card.dart';

class SearchScreen extends ConsumerStatefulWidget {
  const SearchScreen({super.key});

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
    final columns = width >= 650
        ? 5
        : width >= 420
            ? 4
            : 3;
    return Scaffold(
      appBar: AppBar(title: const Text('搜索')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 6, 16, 8),
            child: SearchBar(
              controller: _controller,
              hintText: '搜索漫画、作者或目录名称',
              leading: const Icon(Icons.search_rounded),
              trailing: [
                IconButton(
                  onPressed: _search,
                  icon: const Icon(Icons.arrow_forward_rounded),
                ),
              ],
              onSubmitted: (_) => _search(),
            ),
          ),
          if (_categories.isNotEmpty)
            SizedBox(
              height: 43,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 14),
                children: [
                  for (final item in _categories)
                    Padding(
                      padding: const EdgeInsets.only(right: 7),
                      child: FilterChip(
                        label: Text('${item.name} ${item.count}'),
                        selected: _category == item.slug,
                        onSelected: (selected) {
                          setState(
                              () => _category = selected ? item.slug : null);
                          _search();
                        },
                      ),
                    ),
                ],
              ),
            ),
          if (_tags.isNotEmpty)
            SizedBox(
              height: 43,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 14),
                children: [
                  for (final item in _tags)
                    Padding(
                      padding: const EdgeInsets.only(right: 7),
                      child: FilterChip(
                        label: Text('${item.name} ${item.count}'),
                        selected: _tag == item.name,
                        onSelected: (selected) {
                          setState(() => _tag = selected ? item.name : null);
                          _search();
                        },
                      ),
                    ),
                ],
              ),
            ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _results.isEmpty
                    ? Center(
                        child: Text(_searched ? '没有找到匹配的漫画' : '输入关键词开始搜索'),
                      )
                    : GridView.builder(
                        padding: const EdgeInsets.all(12),
                        gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                          crossAxisCount: columns,
                          childAspectRatio: 0.59,
                          crossAxisSpacing: 10,
                          mainAxisSpacing: 14,
                        ),
                        itemCount: _results.length,
                        itemBuilder: (_, index) => WorkCard(
                          work: _results[index],
                          serverUrl: serverUrl,
                        ),
                      ),
          ),
        ],
      ),
    );
  }
}
