import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'api_client.dart';

class LibrarySummary {
  final String id;
  final String name;
  final String type;
  final int workCount;
  final bool canManage;

  const LibrarySummary({
    required this.id,
    required this.name,
    this.type = 'comic',
    this.workCount = 0,
    this.canManage = false,
  });

  factory LibrarySummary.fromJson(Map<String, dynamic> json) => LibrarySummary(
        id: json['id']?.toString() ?? '',
        name: json['name']?.toString() ?? '',
        type: json['type']?.toString() ?? 'comic',
        workCount: (json['workCount'] ?? json['comicCount']) is num
            ? (json['workCount'] ?? json['comicCount']).toInt()
            : int.tryParse(
                    (json['workCount'] ?? json['comicCount'])?.toString() ??
                        '') ??
                0,
        canManage: json['canManage'] == true,
      );
}

class LibraryApi {
  final Dio _dio;
  LibraryApi(this._dio);

  Future<List<LibrarySummary>> listAccessible() async {
    final response = await _dio.get('/libraries/accessible');
    final data = Map<String, dynamic>.from(response.data as Map);
    return (data['libraries'] as List?)
            ?.whereType<Map>()
            .map((item) =>
                LibrarySummary.fromJson(Map<String, dynamic>.from(item)))
            .where(
                (library) => library.id.isNotEmpty && library.type == 'comic')
            .toList() ??
        const [];
  }
}

final libraryApiProvider = Provider<LibraryApi>((ref) {
  return LibraryApi(ref.watch(dioProvider));
});
