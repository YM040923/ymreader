import 'package:flutter_test/flutter_test.dart';
import 'package:nowen_reader/data/api/work_api.dart';

void main() {
  test('buildWorkQuery maps Android filters to Work API parameters', () {
    final query = buildWorkQuery(const WorkQuery(
      page: 2,
      pageSize: 30,
      sortBy: 'lastReadAt',
      sortOrder: 'desc',
      search: '辉夜',
      libraryIds: ['jp', 'cn'],
      favoritesOnly: true,
      tags: ['校园', '恋爱'],
      category: '日漫',
    ));

    expect(query['page'], 2);
    expect(query['pageSize'], 30);
    expect(query['sortBy'], 'lastReadAt');
    expect(query['sortOrder'], 'desc');
    expect(query['search'], '辉夜');
    expect(query['libraryIds'], 'jp,cn');
    expect(query['favorites'], 'true');
    expect(query['tags'], '校园,恋爱');
    expect(query['category'], '日漫');
  });
}
