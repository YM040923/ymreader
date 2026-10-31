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
  final bool filterByDirectory;
  final String directory;

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
    this.filterByDirectory = false,
    this.directory = '',
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
    bool? filterByDirectory,
    String? directory,
    bool clearDirectory = false,
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
        filterByDirectory: filterByDirectory ?? this.filterByDirectory,
        directory: clearDirectory ? '' : directory ?? this.directory,
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
  if (query.filterByDirectory) {
    result['dir'] = query.directory;
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

class WorkTagStat {
  final String name;
  final String color;
  final int count;

  const WorkTagStat({
    required this.name,
    this.color = '',
    this.count = 0,
  });
}

class WorkCategoryStat {
  final String name;
  final String slug;
  final String icon;
  final int count;

  const WorkCategoryStat({
    required this.name,
    required this.slug,
    this.icon = '',
    this.count = 0,
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

  Future<List<WorkTagStat>> getTagStats() async {
    final response = await _dio.get('/works/tags');
    final data = Map<String, dynamic>.from(response.data as Map);
    return (data['tags'] as List?)
            ?.whereType<Map>()
            .map((item) => WorkTagStat(
                  name: item['name']?.toString() ?? '',
                  color: item['color']?.toString() ?? '',
                  count: _asInt(item['count'], 0),
                ))
            .where((item) => item.name.isNotEmpty)
            .toList() ??
        const [];
  }

  Future<List<WorkCategoryStat>> getCategoryStats() async {
    final response = await _dio.get('/works/categories');
    final data = Map<String, dynamic>.from(response.data as Map);
    return (data['categories'] as List?)
            ?.whereType<Map>()
            .map((item) => WorkCategoryStat(
                  name: item['name']?.toString() ?? '',
                  slug: item['slug']?.toString() ?? '',
                  icon: item['icon']?.toString() ?? '',
                  count: _asInt(item['count'], 0),
                ))
            .where((item) => item.name.isNotEmpty)
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
      data: {'status': value},
    );
  }
  Future<LibraryDirectoryListing> listLibraryDirectories(
    String libraryId, {
    String parent = '',
  }) async {
    final response = await _dio.get(
      '/libraries/${Uri.encodeComponent(libraryId)}/directories',
      queryParameters: {
        if (parent.isNotEmpty) 'parent': parent,
        'pageSize': 200,
      },
    );
    return LibraryDirectoryListing.fromJson(
      Map<String, dynamic>.from(response.data as Map),
    );
  }
}

class LibraryDirectoryNode {
  final String name;
  final String path;
  final int workCount;
  final bool hasChildren;
  final String? coverUrl;
  final String? coverComicId;

  const LibraryDirectoryNode({
    required this.name,
    required this.path,
    required this.workCount,
    required this.hasChildren,
    this.coverUrl,
    this.coverComicId,
  });

  factory LibraryDirectoryNode.fromJson(Map<String, dynamic> json) =>
      LibraryDirectoryNode(
        name: json['name']?.toString() ?? '',
        path: json['path']?.toString() ?? '',
        workCount: _asInt(json['workCount'], 0),
        hasChildren: json['hasChildren'] == true,
        coverUrl: json['coverUrl']?.toString(),
        coverComicId: json['coverComicId']?.toString(),
      );
}

class LibraryDirectoryBreadcrumb {
  final String name;
  final String path;

  const LibraryDirectoryBreadcrumb({required this.name, required this.path});

  factory LibraryDirectoryBreadcrumb.fromJson(Map<String, dynamic> json) =>
      LibraryDirectoryBreadcrumb(
        name: json['name']?.toString() ?? '',
        path: json['path']?.toString() ?? '',
      );
}

class LibraryDirectoryListing {
  final String libraryId;
  final String parent;
  final List<LibraryDirectoryBreadcrumb> breadcrumbs;
  final List<LibraryDirectoryNode> directories;
  final int total;

  const LibraryDirectoryListing({
    required this.libraryId,
    required this.parent,
    required this.breadcrumbs,
    required this.directories,
    required this.total,
  });

  factory LibraryDirectoryListing.fromJson(Map<String, dynamic> json) =>
      LibraryDirectoryListing(
        libraryId: json['libraryId']?.toString() ?? '',
        parent: json['parent']?.toString() ?? '',
        breadcrumbs: (json['breadcrumbs'] as List?)
                ?.whereType<Map>()
                .map((item) => LibraryDirectoryBreadcrumb.fromJson(
                      Map<String, dynamic>.from(item),
                    ))
                .toList() ??
            const [],
        directories: (json['directories'] as List?)
                ?.whereType<Map>()
                .map((item) => LibraryDirectoryNode.fromJson(
                      Map<String, dynamic>.from(item),
                    ))
                .toList() ??
            const [],
        total: _asInt(json['total'], 0),
      );
}

int _asInt(dynamic value, int fallback) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

final workApiProvider = Provider<WorkApi>((ref) {
  return WorkApi(ref.watch(dioProvider));
});
