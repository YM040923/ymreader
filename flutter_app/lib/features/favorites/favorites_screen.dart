import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api/work_api.dart';
import '../../data/models/work.dart';
import '../../data/providers/auth_provider.dart';
import '../../widgets/work_card.dart';

class FavoritesScreen extends ConsumerStatefulWidget {
  const FavoritesScreen({super.key});

  @override
  ConsumerState<FavoritesScreen> createState() => _FavoritesScreenState();
}

class _FavoritesScreenState extends ConsumerState<FavoritesScreen> {
  List<Work> _works = const [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final response = await ref.read(workApiProvider).listWorks(
            const WorkQuery(
              pageSize: 200,
              favoritesOnly: true,
              sortBy: 'title',
            ),
          );
      if (!mounted) return;
      setState(() {
        _works = response.works;
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
      appBar: AppBar(title: const Text('收藏')),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _works.isEmpty
                ? ListView(
                    children: const [
                      SizedBox(height: 220),
                      Center(child: Text('还没有收藏漫画')),
                    ],
                  )
                : GridView.builder(
                    padding: const EdgeInsets.all(12),
                    gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                      crossAxisCount: columns,
                      childAspectRatio: 0.59,
                      crossAxisSpacing: 10,
                      mainAxisSpacing: 14,
                    ),
                    itemCount: _works.length,
                    itemBuilder: (_, index) {
                      final work = _works[index];
                      return WorkCard(
                        work: work,
                        serverUrl: serverUrl,
                        onFavorite: () async {
                          await ref
                              .read(workApiProvider)
                              .setFavorite(work.id, false);
                          await _load();
                        },
                      );
                    },
                  ),
      ),
    );
  }
}
