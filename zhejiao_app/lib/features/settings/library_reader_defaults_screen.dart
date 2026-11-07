import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/providers/work_provider.dart';
import '../../widgets/reader_settings_panel.dart';

class LibraryReaderDefaultsScreen extends ConsumerStatefulWidget {
  const LibraryReaderDefaultsScreen({super.key});

  @override
  ConsumerState<LibraryReaderDefaultsScreen> createState() =>
      _LibraryReaderDefaultsScreenState();
}

class _LibraryReaderDefaultsScreenState
    extends ConsumerState<LibraryReaderDefaultsScreen> {
  final Map<String, ReaderSettings> _settings = {};

  Future<ReaderSettings> _load(String libraryId) async {
    return _settings[libraryId] ??=
        await ReaderSettings.load(scope: 'library:$libraryId');
  }

  String _summary(ReaderSettings settings) {
    final mode = switch (settings.mode) {
      ComicReadingMode.single => '单页',
      ComicReadingMode.doublePage => '双页',
      ComicReadingMode.webtoon => '长条',
    };
    final direction = switch (settings.direction) {
      ReadingDirection.ltr => '左→右',
      ReadingDirection.rtl => '右→左',
      ReadingDirection.ttb => '上→下',
    };
    return '$mode · $direction'
        '${settings.continuousReading ? ' · 连续阅读' : ''}';
  }

  Future<void> _edit(String libraryId) async {
    final settings = await _load(libraryId);
    if (!mounted) return;
    ReaderSettingsPanel.show(
      context,
      settings: settings,
      scope: 'library:$libraryId',
      onChanged: (value) => setState(() => _settings[libraryId] = value),
    );
  }

  @override
  Widget build(BuildContext context) {
    final libraries = ref.watch(accessibleLibrariesProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('漫画库默认阅读设置')),
      body: libraries.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(child: Text('加载失败：$error')),
        data: (items) => ListView.separated(
          padding: const EdgeInsets.symmetric(vertical: 8),
          itemCount: items.length,
          separatorBuilder: (_, __) => const Divider(height: 1),
          itemBuilder: (_, index) {
            final library = items[index];
            return FutureBuilder<ReaderSettings>(
              future: _load(library.id),
              builder: (_, snapshot) => ListTile(
                leading: const CircleAvatar(
                  child: Icon(Icons.local_library_rounded),
                ),
                title: Text(library.name),
                subtitle: Text(
                  snapshot.hasData ? _summary(snapshot.data!) : '正在读取',
                ),
                trailing: const Icon(Icons.chevron_right_rounded),
                onTap: () => _edit(library.id),
              ),
            );
          },
        ),
      ),
    );
  }
}
