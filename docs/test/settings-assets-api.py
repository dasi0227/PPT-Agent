"""Verify Java asset partial updates against start-settings-assets.sh (Python 3)."""
import json
import os
from urllib.parse import urlencode
from urllib.request import Request, urlopen
from uuid import uuid4

base = 'http://127.0.0.1:' + os.getenv('FRONTEND_PORT', '15173') + '/api'


def call(path, data=None):
    request = Request(base + path, data=None if data is None else json.dumps(data).encode(),
                      headers={'Content-Type': 'application/json'})
    with urlopen(request, timeout=15) as response:
        assert response.status == 200
        raw = response.read()
    return json.loads(raw) if raw else None


for kind in ('theme', 'component', 'skill', 'snippet'):
    name = 'api-' + uuid4().hex
    tag = call('/assets/tags/create', {'name': name, 'type': kind})
    asset = call('/assets/create', {'type': kind, 'name': name, 'description': 'api check',
                                    'content': 'original', 'tag_ids': [tag['id']]})
    asset_id = asset['id']
    try:
        rows = call('/assets?' + urlencode({'type': kind, 'query': name}))['items']
        assert [row['id'] for row in rows] == [asset_id] and rows[0]['hash'] is None
        updated = call('/assets/update', {'asset_id': asset_id, 'enabled': False,
                                          'description': None, 'tag_ids': None})
        assert updated['enabled'] is False and updated['tag_ids'] == [tag['id']]
        assert updated['description'] == asset['description'] and updated['hash'] == asset['hash']
        assert call('/assets/tags/delete', {'tag_id': tag['id']}) is None
        tag = None
        assert call('/assets?' + urlencode({'type': kind, 'query': name}))['items'][0]['tag_ids'] == []
        body = call('/assets/' + asset_id)
        assert body == {'content': 'original', 'hash': updated['hash']}
        assert call('/assets/update', {'asset_id': asset_id, 'content': 'unpaired'})['error']['code'] == 'INVALID_ARGUMENT'
    finally:
        assert call('/assets/delete', {'asset_id': asset_id}) is None
        if tag:
            call('/assets/tags/delete', {'tag_id': tag['id']})
    print('PASS', kind, 'null/false, tag cascade, hash, empty success, business errors')
