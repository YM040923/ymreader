import 'comic.dart';

int _asInt(dynamic value, [int fallback = 0]) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

double? _asDouble(dynamic value) {
  if (value is num) return value.toDouble();
  return double.tryParse(value?.toString() ?? '');
}

class WorkUnit {
  final String id;
  final String workId;
  final String comicId;
  final String title;
  final String displayLabel;
  final String relativePath;
  final String internalPath;
  final int startPage;
  final int pageCount;
  final int coverPage;
  final int sortIndex;
  final String? coverUrl;
  final double coverAspectRatio;
  final int lastReadPage;
  final String? lastReadAt;
  final String readingStatus;

  const WorkUnit({
    required this.id,
    required this.workId,
    required this.comicId,
    required this.title,
    required this.displayLabel,
    this.relativePath = '',
    this.internalPath = '',
    this.startPage = 0,
    this.pageCount = 0,
    this.coverPage = 0,
    this.sortIndex = 0,
    this.coverUrl,
    this.coverAspectRatio = 0,
    this.lastReadPage = 0,
    this.lastReadAt,
    this.readingStatus = '',
  });

  factory WorkUnit.fromJson(Map<String, dynamic> json) => WorkUnit(
        id: json['id']?.toString() ?? '',
        workId: json['workId']?.toString() ?? '',
        comicId: json['comicId']?.toString() ?? '',
        title: json['title']?.toString() ?? '',
        displayLabel:
            json['displayLabel']?.toString() ?? json['title']?.toString() ?? '',
        relativePath: json['relativePath']?.toString() ?? '',
        internalPath: json['internalPath']?.toString() ?? '',
        startPage: _asInt(json['startPage']),
        pageCount: _asInt(json['pageCount']),
        coverPage: _asInt(json['coverPage']),
        sortIndex: _asInt(json['sortIndex']),
        coverUrl: json['coverUrl']?.toString(),
        coverAspectRatio: _asDouble(json['coverAspectRatio']) ?? 0,
        lastReadPage: _asInt(json['lastReadPage']),
        lastReadAt: json['lastReadAt']?.toString(),
        readingStatus: json['readingStatus']?.toString() ?? '',
      );

  int get endPage => pageCount > 0 ? startPage + pageCount - 1 : startPage;

  bool containsAbsolutePage(int page) => page >= startPage && page <= endPage;

  int get resolvedCoverPage =>
      coverPage >= startPage && coverPage <= endPage ? coverPage : startPage;

  String resolvedCoverUrl(String serverUrl) {
    final base = serverUrl.replaceFirst(RegExp(r'/$'), '');
    final raw = coverUrl?.trim() ?? '';
    if (raw.startsWith('http://') || raw.startsWith('https://')) return raw;
    if (raw.isNotEmpty) return '$base${raw.startsWith('/') ? raw : '/$raw'}';
    return '$base/api/comics/${Uri.encodeComponent(comicId)}/page/'
        '$resolvedCoverPage';
  }
}

class WorkReadingTarget {
  final WorkUnit unit;
  final int absolutePage;
  final bool isContinue;

  const WorkReadingTarget({
    required this.unit,
    required this.absolutePage,
    required this.isContinue,
  });

  int get relativePage =>
      (absolutePage - unit.startPage).clamp(0, unit.pageCount - 1);
}

class Work {
  final String id;
  final String libraryId;
  final String title;
  final String rootPath;
  final String representativeComicId;
  final String coverComicId;
  final String? coverUrl;
  final double coverAspectRatio;
  final int itemCount;
  final int completedItemCount;
  final int pageCount;
  final int fileSize;
  final int totalReadTime;
  final String addedAt;
  final String updatedAt;
  final int sortOrder;
  final String author;
  final String publisher;
  final int? year;
  final String description;
  final String language;
  final String genre;
  final String status;
  final String metadataSource;
  final double? externalRating;
  final double externalRatingMax;
  final String externalRatingSource;
  final List<Tag> tags;
  final List<Category> categories;
  final bool isFavorite;
  final int? rating;
  final String readingStatus;
  final String? lastReadAt;
  final String continueComicId;
  final int continuePage;
  final String continueUnitId;
  final List<WorkUnit> units;

  const Work({
    required this.id,
    required this.title,
    this.libraryId = '',
    this.rootPath = '',
    this.representativeComicId = '',
    this.coverComicId = '',
    this.coverUrl,
    this.coverAspectRatio = 0,
    this.itemCount = 0,
    this.completedItemCount = 0,
    this.pageCount = 0,
    this.fileSize = 0,
    this.totalReadTime = 0,
    this.addedAt = '',
    this.updatedAt = '',
    this.sortOrder = 0,
    this.author = '',
    this.publisher = '',
    this.year,
    this.description = '',
    this.language = '',
    this.genre = '',
    this.status = '',
    this.metadataSource = '',
    this.externalRating,
    this.externalRatingMax = 0,
    this.externalRatingSource = '',
    this.tags = const [],
    this.categories = const [],
    this.isFavorite = false,
    this.rating,
    this.readingStatus = '',
    this.lastReadAt,
    this.continueComicId = '',
    this.continuePage = 0,
    this.continueUnitId = '',
    this.units = const [],
  });

  factory Work.fromJson(Map<String, dynamic> json) {
    final units = (json['units'] as List?)
            ?.whereType<Map>()
            .map((item) => WorkUnit.fromJson(Map<String, dynamic>.from(item)))
            .where((unit) => unit.id.isNotEmpty && unit.comicId.isNotEmpty)
            .toList() ??
        <WorkUnit>[];
    units.sort((left, right) {
      final byIndex = left.sortIndex.compareTo(right.sortIndex);
      if (byIndex != 0) return byIndex;
      return left.displayLabel.compareTo(right.displayLabel);
    });
    return Work(
      id: json['id']?.toString() ?? '',
      libraryId: json['libraryId']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      rootPath: json['rootPath']?.toString() ?? '',
      representativeComicId: json['representativeComicId']?.toString() ?? '',
      coverComicId: json['coverComicId']?.toString() ?? '',
      coverUrl: json['coverUrl']?.toString(),
      coverAspectRatio: _asDouble(json['coverAspectRatio']) ?? 0,
      itemCount: _asInt(json['itemCount']),
      completedItemCount: _asInt(json['completedItemCount']),
      pageCount: _asInt(json['pageCount']),
      fileSize: _asInt(json['fileSize']),
      totalReadTime: _asInt(json['totalReadTime']),
      addedAt: json['addedAt']?.toString() ?? '',
      updatedAt: json['updatedAt']?.toString() ?? '',
      sortOrder: _asInt(json['sortOrder']),
      author: json['author']?.toString() ?? '',
      publisher: json['publisher']?.toString() ?? '',
      year: json['year'] == null ? null : _asInt(json['year']),
      description: json['description']?.toString() ?? '',
      language: json['language']?.toString() ?? '',
      genre: json['genre']?.toString() ?? '',
      status: json['status']?.toString() ?? '',
      metadataSource: json['metadataSource']?.toString() ?? '',
      externalRating: _asDouble(json['externalRating']),
      externalRatingMax: _asDouble(json['externalRatingMax']) ?? 0,
      externalRatingSource: json['externalRatingSource']?.toString() ?? '',
      tags: (json['tags'] as List?)
              ?.whereType<Map>()
              .map((item) => Tag.fromJson(Map<String, dynamic>.from(item)))
              .toList() ??
          const [],
      categories: (json['categories'] as List?)
              ?.whereType<Map>()
              .map((item) => Category.fromJson(Map<String, dynamic>.from(item)))
              .toList() ??
          const [],
      isFavorite: json['isFavorite'] == true,
      rating: json['rating'] == null ? null : _asInt(json['rating']),
      readingStatus: json['readingStatus']?.toString() ?? '',
      lastReadAt: json['lastReadAt']?.toString(),
      continueComicId: json['continueComicId']?.toString() ?? '',
      continuePage: _asInt(json['continuePage']),
      continueUnitId: json['continueUnitId']?.toString() ?? '',
      units: units,
    );
  }

  bool get hasReadingProgress =>
      continueUnitId.isNotEmpty ||
      continueComicId.isNotEmpty ||
      (lastReadAt?.isNotEmpty ?? false) ||
      readingStatus == 'reading' ||
      readingStatus == 'finished';

  WorkReadingTarget? get continueTarget {
    if (!hasReadingProgress || units.isEmpty) return null;
    WorkUnit? unit;
    for (final candidate in units) {
      if (candidate.id == continueUnitId) {
        unit = candidate;
        break;
      }
    }
    unit ??= units.cast<WorkUnit?>().firstWhere(
          (candidate) =>
              candidate!.comicId == continueComicId &&
              candidate.containsAbsolutePage(continuePage),
          orElse: () => null,
        );
    unit ??= units.cast<WorkUnit?>().firstWhere(
          (candidate) => candidate!.comicId == continueComicId,
          orElse: () => null,
        );
    if (unit == null) return null;
    return WorkReadingTarget(
      unit: unit,
      absolutePage: continuePage.clamp(unit.startPage, unit.endPage),
      isContinue: true,
    );
  }

  WorkReadingTarget? get readingTarget =>
      continueTarget ??
      (units.isEmpty
          ? null
          : WorkReadingTarget(
              unit: units.first,
              absolutePage: units.first.startPage,
              isContinue: false,
            ));

  int get progress {
    final target = continueTarget;
    if (target == null || pageCount <= 0) return 0;
    final completedBefore = units
        .takeWhile((unit) => unit.id != target.unit.id)
        .fold<int>(0, (sum, unit) => sum + unit.pageCount);
    return (((completedBefore + target.relativePage + 1) / pageCount) * 100)
        .round()
        .clamp(0, 100);
  }

  String readerRoute({WorkReadingTarget? target}) {
    final selected = target ?? readingTarget;
    if (selected == null) return '/work/${Uri.encodeComponent(id)}';
    final query = Uri(queryParameters: {
      'page': selected.absolutePage.toString(),
      'workId': id,
      'unitId': selected.unit.id,
    }).query;
    return '/reader/${Uri.encodeComponent(selected.unit.comicId)}?$query';
  }

  String detailRoute() => '/work/${Uri.encodeComponent(id)}';

  String resolvedCoverUrl(String serverUrl) {
    final raw = coverUrl?.trim() ?? '';
    if (raw.startsWith('http://') || raw.startsWith('https://')) return raw;
    if (raw.isNotEmpty) {
      return '${serverUrl.replaceFirst(RegExp(r'/$'), '')}'
          '${raw.startsWith('/') ? raw : '/$raw'}';
    }
    if (coverComicId.isNotEmpty) {
      return '${serverUrl.replaceFirst(RegExp(r'/$'), '')}'
          '/api/comics/${Uri.encodeComponent(coverComicId)}/thumbnail';
    }
    return '';
  }
}
