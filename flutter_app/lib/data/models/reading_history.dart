int _int(dynamic value) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? 0;
}

class ReadingHistoryItem {
  final String id;
  final String contentType;
  final String title;
  final String coverUrl;
  final String genre;
  final int totalReadTime;
  final int sessionCount;
  final String lastReadAt;
  final String lastComicId;
  final String lastUnitId;
  final int startPage;
  final int endPage;
  final String detailHref;
  final String readHref;

  const ReadingHistoryItem({
    required this.id,
    this.contentType = 'comic',
    required this.title,
    this.coverUrl = '',
    this.genre = '',
    this.totalReadTime = 0,
    this.sessionCount = 0,
    this.lastReadAt = '',
    this.lastComicId = '',
    this.lastUnitId = '',
    this.startPage = 0,
    this.endPage = 0,
    this.detailHref = '',
    this.readHref = '',
  });

  bool get isNovel => contentType == 'novel';

  factory ReadingHistoryItem.fromJson(Map<String, dynamic> json) {
    return ReadingHistoryItem(
      id: json['id']?.toString() ?? '',
      contentType: json['contentType']?.toString() ?? 'comic',
      title: json['title']?.toString() ?? '',
      coverUrl: json['coverUrl']?.toString() ?? '',
      genre: json['genre']?.toString() ?? '',
      totalReadTime: _int(json['totalReadTime']),
      sessionCount: _int(json['sessionCount']),
      lastReadAt: json['lastReadAt']?.toString() ?? '',
      lastComicId: json['lastComicId']?.toString() ?? '',
      lastUnitId: json['lastUnitId']?.toString() ?? '',
      startPage: _int(json['startPage']),
      endPage: _int(json['endPage']),
      detailHref: json['detailHref']?.toString() ?? '',
      readHref: json['readHref']?.toString() ?? '',
    );
  }

  String resolvedCoverUrl(String serverUrl) {
    final base = serverUrl.replaceFirst(RegExp(r'/$'), '');
    if (coverUrl.startsWith('http://') || coverUrl.startsWith('https://')) {
      return coverUrl;
    }
    if (coverUrl.isNotEmpty) {
      return '$base${coverUrl.startsWith('/') ? coverUrl : '/$coverUrl'}';
    }
    if (lastComicId.isNotEmpty) {
      return '$base/api/comics/${Uri.encodeComponent(lastComicId)}/thumbnail';
    }
    return '';
  }

  String get route {
    if (detailHref.isNotEmpty) return detailHref;
    return isNovel ? '/comic/$id' : '/work/$id';
  }
}
