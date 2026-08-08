enum FolderStatsScope { work, physical }

int _asInt(dynamic value) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? 0;
}

class FolderStatsResponse {
  final List<FolderStatsNode> roots;

  const FolderStatsResponse({this.roots = const []});

  factory FolderStatsResponse.fromJson(dynamic json) {
    if (json is! List) {
      throw const FormatException(
        'Folder statistics must be returned as a JSON array',
      );
    }
    return FolderStatsResponse(
      roots: json
          .whereType<Map>()
          .map(
            (item) => FolderStatsNode.fromJson(
              Map<String, dynamic>.from(item),
            ),
          )
          .toList(growable: false),
    );
  }
}

class FolderStatsNode {
  final String name;
  final String path;
  final int fileCount;
  final int totalSize;
  final int totalPages;
  final List<FolderStatsNode> children;
  final List<FolderStatsFile> files;

  const FolderStatsNode({
    required this.name,
    required this.path,
    this.fileCount = 0,
    this.totalSize = 0,
    this.totalPages = 0,
    this.children = const [],
    this.files = const [],
  });

  factory FolderStatsNode.fromJson(Map<String, dynamic> json) {
    final rawChildren = json['children'];
    final rawFiles = json['files'];
    return FolderStatsNode(
      name: json['name']?.toString() ?? '',
      path: json['path']?.toString() ?? '',
      fileCount: _asInt(json['fileCount'] ?? json['count']),
      totalSize: _asInt(json['totalSize'] ?? json['size']),
      totalPages: _asInt(json['totalPages']),
      children: rawChildren is List
          ? rawChildren
              .whereType<Map>()
              .map(
                (item) => FolderStatsNode.fromJson(
                  Map<String, dynamic>.from(item),
                ),
              )
              .toList(growable: false)
          : const [],
      files: rawFiles is List
          ? rawFiles
              .whereType<Map>()
              .map(
                (item) => FolderStatsFile.fromJson(
                  Map<String, dynamic>.from(item),
                ),
              )
              .toList(growable: false)
          : const [],
    );
  }
}

class FolderStatsFile {
  final String id;
  final String title;
  final String filename;
  final int fileSize;
  final int pageCount;
  final String type;
  final int lastRead;

  const FolderStatsFile({
    required this.id,
    required this.title,
    required this.filename,
    this.fileSize = 0,
    this.pageCount = 0,
    this.type = 'comic',
    this.lastRead = 0,
  });

  factory FolderStatsFile.fromJson(Map<String, dynamic> json) =>
      FolderStatsFile(
        id: json['id']?.toString() ?? '',
        title: json['title']?.toString() ?? '',
        filename: json['filename']?.toString() ?? '',
        fileSize: _asInt(json['fileSize']),
        pageCount: _asInt(json['pageCount']),
        type: json['type']?.toString() ?? 'comic',
        lastRead: _asInt(json['lastRead']),
      );
}

class FileStatsSummary {
  final int totalFiles;
  final int totalSize;
  final int totalPages;
  final int comicCount;
  final int novelCount;

  const FileStatsSummary({
    this.totalFiles = 0,
    this.totalSize = 0,
    this.totalPages = 0,
    this.comicCount = 0,
    this.novelCount = 0,
  });

  factory FileStatsSummary.fromJson(dynamic json) {
    if (json is! Map) {
      throw const FormatException(
        'File statistics must be returned as a JSON object',
      );
    }
    final data = Map<String, dynamic>.from(json);
    return FileStatsSummary(
      totalFiles: _asInt(data['totalFiles']),
      totalSize: _asInt(data['totalSize']),
      totalPages: _asInt(data['totalPages']),
      comicCount: _asInt(data['comicCount']),
      novelCount: _asInt(data['novelCount']),
    );
  }
}
