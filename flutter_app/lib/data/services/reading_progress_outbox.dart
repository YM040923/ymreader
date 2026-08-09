import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

class ReadingProgressEvent {
  final String serverKey;
  final String userKey;
  final String comicId;
  final String? workId;
  final String? unitId;
  final int? relativePage;
  final int page;
  final int totalPages;
  final int activeSeconds;
  final String clientSessionId;
  final int sequence;
  final bool finalize;
  final bool trackProgress;

  const ReadingProgressEvent({
    required this.serverKey,
    required this.userKey,
    required this.comicId,
    required this.page,
    required this.totalPages,
    required this.activeSeconds,
    required this.clientSessionId,
    required this.sequence,
    this.workId,
    this.unitId,
    this.relativePage,
    this.finalize = false,
    this.trackProgress = true,
  });

  String get storageKey =>
      '$serverKey\u0000$userKey\u0000${workId ?? 'comic:$comicId'}';

  ReadingProgressEvent copyWith({
    String? comicId,
    String? workId,
    String? unitId,
    int? relativePage,
    int? page,
    int? totalPages,
    int? activeSeconds,
    int? sequence,
    bool? finalize,
    bool? trackProgress,
  }) =>
      ReadingProgressEvent(
        serverKey: serverKey,
        userKey: userKey,
        comicId: comicId ?? this.comicId,
        workId: workId ?? this.workId,
        unitId: unitId ?? this.unitId,
        relativePage: relativePage ?? this.relativePage,
        page: page ?? this.page,
        totalPages: totalPages ?? this.totalPages,
        activeSeconds: activeSeconds ?? this.activeSeconds,
        clientSessionId: clientSessionId,
        sequence: sequence ?? this.sequence,
        finalize: finalize ?? this.finalize,
        trackProgress: trackProgress ?? this.trackProgress,
      );

  Map<String, dynamic> toJson() => {
        'serverKey': serverKey,
        'userKey': userKey,
        'comicId': comicId,
        'workId': workId,
        'unitId': unitId,
        'relativePage': relativePage,
        'page': page,
        'totalPages': totalPages,
        'activeSeconds': activeSeconds,
        'clientSessionId': clientSessionId,
        'sequence': sequence,
        'finalize': finalize,
        'trackProgress': trackProgress,
      };

  factory ReadingProgressEvent.fromJson(Map<String, dynamic> json) =>
      ReadingProgressEvent(
        serverKey: json['serverKey'] as String? ?? 'default',
        userKey: json['userKey'] as String? ?? 'default',
        comicId: json['comicId'] as String,
        workId: json['workId'] as String?,
        unitId: json['unitId'] as String?,
        relativePage: json['relativePage'] as int?,
        page: json['page'] as int? ?? 0,
        totalPages: json['totalPages'] as int? ?? 0,
        activeSeconds: json['activeSeconds'] as int? ?? 0,
        clientSessionId: json['clientSessionId'] as String,
        sequence: json['sequence'] as int? ?? 0,
        finalize: json['finalize'] == true,
        trackProgress: json['trackProgress'] != false,
      );
}

class ReadingProgressOutbox {
  static const _key = 'reading_progress_outbox_v1';

  Future<Map<String, ReadingProgressEvent>> _readMap() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(_key);
    if (raw == null || raw.isEmpty) return {};
    final decoded = jsonDecode(raw) as Map<String, dynamic>;
    return decoded.map((key, value) => MapEntry(
          key,
          ReadingProgressEvent.fromJson(
              Map<String, dynamic>.from(value as Map)),
        ));
  }

  Future<void> _writeMap(Map<String, ReadingProgressEvent> values) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      _key,
      jsonEncode(values.map((key, value) => MapEntry(key, value.toJson()))),
    );
  }

  Future<void> put(ReadingProgressEvent event) async {
    final values = await _readMap();
    values[event.storageKey] = event;
    await _writeMap(values);
  }

  Future<void> remove(ReadingProgressEvent event) async {
    final values = await _readMap();
    values.remove(event.storageKey);
    await _writeMap(values);
  }

  Future<List<ReadingProgressEvent>> readAll() async =>
      (await _readMap()).values.toList();
}
