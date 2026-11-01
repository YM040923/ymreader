import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/api/work_api.dart';

void main() {
  test('manual Work mutations match the server routes and payloads', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final requests = <Map<String, dynamic>>[];
    final subscription = server.listen((request) async {
      final body = await utf8.decoder.bind(request).join();
      requests.add({
        'method': request.method,
        'path': request.uri.path,
        'body': body,
      });
      request.response.headers.contentType = ContentType.json;
      if (request.uri.path.endsWith('/split')) {
        request.response.write(jsonEncode({'workId': 'work_manual_new'}));
      } else {
        request.response.write(jsonEncode({'success': true}));
      }
      await request.response.close();
    });

    final dio = Dio(BaseOptions(
      baseUrl: 'http://127.0.0.1:${server.port}/api',
      headers: {'Content-Type': 'application/json'},
    ));
    final api = WorkApi(dio);
    try {
      await api.mergeWorks(['work_a', 'work_b']);
      final newWorkId =
          await api.splitWork('work_a', ['unit_1', 'unit_2'], title: '拆分');
      await api.restoreWorkAutoGrouping('work_a');

      expect(newWorkId, 'work_manual_new');
      expect(requests.map((request) => '${request['method']} ${request['path']}'), [
        'POST /api/works/merge',
        'POST /api/works/work_a/split',
        'POST /api/works/work_a/restore-auto',
      ]);
      expect(jsonDecode(requests[0]['body'] as String), {
        'workIds': ['work_a', 'work_b'],
      });
      expect(jsonDecode(requests[1]['body'] as String), {
        'unitIds': ['unit_1', 'unit_2'],
        'title': '拆分',
      });
      expect(requests[2]['body'], isEmpty);
    } finally {
      await subscription.cancel();
      await server.close(force: true);
    }
  });
}
