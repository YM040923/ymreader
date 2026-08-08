import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/work.dart';
import 'api_client.dart';

class WorkQuery {
  final int page;
  final int pageSize;
  final String sortBy;
  final String sortOrder;
  final String? search;
  final List<String> libraryIds;
  final bool favoritesOnly;
  final List<String> tags;
  final String? category;
  final String? readingStatus;

  const WorkQuery({
    this.page = 1,
    this.pageSize = 30,
    this.sortBy = 'title',
    this.sortOrder = 'asc',
    this.search,
    this.libraryIds = const [],
    this.favoritesOnly = false,
    this.tags = const [],
    this.category,
    this.readingStatus,
  });

  WorkQuery copyWith({
    int? page,
    int? pageSize,
    String? sortBy,
    String? sortOrder,
    String? search,
    bool clearSearch = false,
    List<String>? libraryIds,
    bool? favoritesOnly,
    List<String>? tags,
    String? category,
    bool clearCategory = false,
    String? readingStatus,
  }) =>
      WorkQuery(
        page: page ?? this.page,
        pageSize: pageSize ?? this.pageSize,
        sortBy: sortBy ?? this.sortBy,
        sortOrder: sortOrder ?? this.sortOrder,
        search: clearSearch ? null : search ?? this.search,
        libraryIds: libraryIds ?? this.libraryIds,
        favoritesOnly: favoritesOnly ?? this.favoritesOnly,
        tags: tags ?? this.tags,
        category: clearCategory ? null : category ?? this.category,
        readingStatus: readingStatus ?? this.readingStatus,
      );
}

Map<String, dynamic> buildWorkQuery(WorkQuery query) {
  final result = <String, dynamic>{
    'page': query.page,
    'pageSize': query.pageSize,
    'sortBy': query.sortBy,
    'sortOrder': query.sortOrder,
  };
  if (query.search?.trim().isNotEmpty == true) {
    result['search'] = query.search!.trim();
  }
  if (query.libraryIds.isNotEmpty) {
    result['libraryIds'] = query.libraryIds.join(',');
  }
  if (query.favoritesOnly) result['favorites'] = 'true';
  if (query.tags.isNotEmpty) result['tags'] = query.tags.join(',');
  if (query.category?.trim().isNotEmpty == true) {
    result['category'] = query.category!.trim();
  }
  if (query.readingStatus?.trim().isNotEmpty == true) {
    result['readingStatus'] = query.readingStatus!.trim();
  }
  return result;
}

class WorkListResponse {
  final List<Work> works;
  final int total;
  final int page;
  final int pageSize;
  final int totalPages;

  const WorkListResponse({
    required this.works,
    required this.total,
    required this.page,
    required this.pageSize,
    required this.totalPages,
  });
}

class WorkApi {
  final Dio _dio;

  WorkApi(this._dio);

  Future<WorkListResponse> listWorks(
      [WorkQuery query = const WorkQuery()]) async {
    final response =
        await _dio.get('/works', queryParameters: buildWorkQuery(query));
    final data = Map<String, dynamic>.from(response.data as Map);
    final works = (data['works'] as List?)
            ?.whereType<Map>()
            .map((item) => Work.fromJson(Map<String, dynamic>.from(item)))
            .toList() ??
        const <Work>[];
    return WorkListResponse(
      works: works,
      total: _asInt(data['total'], works.length),
      page: _asInt(data['page'], query.page),
      pageSize: _asInt(data['pageSize'], query.pageSize),
      totalPages: _asInt(data['totalPages'], 1),
    );
  }

  Future<Work> getWork(String id) async {
    final response = await _dio.get('/works/${Uri.encodeComponent(id)}');
    return Work.fromJson(Map<String, dynamic>.from(response.data as Map));
  }

  Future<List<WorkUnit>> getUnits(String id) async {
    final response = await _dio.get('/works/${Uri.encodeComponent(id)}/units');
    final data = Map<String, dynamic>.from(response.data as Map);
    return (data['units'] as List?)
            ?.whereType<Map>()
            .map((item) => WorkUnit.fromJson(Map<String, dynamic>.from(item)))
            .toList() ??
        const [];
  }

  Future<void> setFavorite(String id, bool value) async {
    await _dio.put(
      '/works/${Uri.encodeComponent(id)}/favorite',
      data: {'isFavorite': value},
    );
  }

  Future<void> setRating(String id, int? value) async {
    await _dio.put(
      '/works/${Uri.encodeComponent(id)}/rating',
      data: {'rating': value},
    );
  }

  Future<void> setReadingStatus(String id, String value) async {
    await _dio.put(
      '/works/${Uri.encodeComponent(id)}/reading-status',
      data: {'readingStatus': value},
    );
  }
}

int _asInt(dynamic value, int fallback) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

final workApiProvider = Provider<WorkApi>((ref) {
  return WorkApi(ref.watch(dioProvider));
});
