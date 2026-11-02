import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/api/comic_api.dart';

/// Answers the first activity call with the same 400 the server returns when it
/// refuses a Work cursor, and every later call with a plain success.
class _CursorRejectingAdapter implements HttpClientAdapter {
  final List<Map<String, dynamic>> bodies = [];
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    bodies.add(Map<String, dynamic>.from(options.data as Map));
    if (bodies.length == 1) {
      return ResponseBody.fromString(
        '{"error":"Invalid Work reading cursor","reason":"Comic does not belong to Work"}',
        400,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString(
      '{"success":true,"page":3,"totalPages":539}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('a rejected Work cursor falls back to Comic-only progress', () async {
    final adapter = _CursorRejectingAdapter();
    final api = ComicApi(Dio(BaseOptions(baseUrl: 'http://test/api'))
      ..httpClientAdapter = adapter);

    final result = await api.recordReadingActivity(
      comicId: 'f49661694588',
      clientSessionId: 'session',
      page: 3,
      totalPages: 539,
      activeSeconds: 12,
      sequence: 2,
      workId: 'work_e2ad2617dd8688768554',
      unitId: 'unit_c65e064679eda9a79e82',
      relativePage: 3,
    );

    expect(adapter.bodies, hasLength(2));
    expect(adapter.bodies.first['workId'], isNotNull);
    expect(adapter.bodies.last.containsKey('workId'), isFalse);
    expect(adapter.bodies.last.containsKey('unitId'), isFalse);
    expect(adapter.bodies.last['page'], 3);
    expect(result['cursorRejected'], isTrue);
    expect(result['page'], 3);
  });

  test('other failures still surface instead of being swallowed', () async {
    final api = ComicApi(Dio(BaseOptions(baseUrl: 'http://test/api'))
      ..httpClientAdapter = _AlwaysFailingAdapter());

    expect(
      () => api.recordReadingActivity(
        comicId: 'c',
        clientSessionId: 's',
        page: 1,
        totalPages: 10,
        activeSeconds: 1,
        sequence: 1,
        workId: 'w',
        unitId: 'u',
        relativePage: 1,
      ),
      throwsA(isA<DioException>()),
    );
  });
}

class _AlwaysFailingAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString('{"error":"boom"}', 500, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}
