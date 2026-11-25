"""Verify Java asset and model write responses and detail reads against start-settings-assets.sh (Python 3)."""
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
        assert [row['id'] for row in rows] == [asset_id]
        assert set(tag) == set(asset) == {'id'}
        assert 'content' not in rows[0]
        assert call('/assets/' + asset_id) == {**rows[0], 'content': 'original'}
        assert call('/assets/update', {'asset_id': asset_id, 'type': kind, 'enabled': False,
                                       'description': None, 'tag_ids': None}) is None
        updated = call('/assets/' + asset_id)
        assert updated['enabled'] is False and updated['tag_ids'] == [tag['id']]
        assert updated['description'] == rows[0]['description']
        assert call('/assets/tags/delete', {'tag_id': tag['id']}) is None
        tag = None
        assert call('/assets?' + urlencode({'type': kind, 'query': name}))['items'][0]['tag_ids'] == []
        assert call('/assets/' + asset_id)['tag_ids'] == []
        assert call('/assets/update', {'asset_id': asset_id, 'type': kind, 'content': 'edited'}) is None
        assert call('/assets/' + asset_id)['content'] == 'edited'
    finally:
        assert call('/assets/delete', {'asset_id': asset_id}) is None
        if tag:
            call('/assets/tags/delete', {'tag_id': tag['id']})
    assert call('/assets/' + asset_id)['error']['code'] == 'ASSET_NOT_FOUND'
    print('PASS', kind, 'null/false, tag cascade, content update, empty success')


name = 'api-' + uuid4().hex
model = call('/settings/models/create', {'name': name, 'protocol': 'responses', 'provider': 'gpt',
                                        'base_url': 'https://example.invalid/v1///', 'model': 'test', 'key': 'test-only'})
model_id = model['id']
try:
    assert set(model) == {'id'}
    detail = call('/settings/models/' + model_id)
    assert detail == {'id': model_id, 'name': name, 'protocol': 'responses', 'provider': 'gpt',
                      'base_url': 'https://example.invalid/v1', 'model': 'test'}
    assert detail in call('/settings/models')['models']
    assert call('/settings/models/update', {'id': model_id, 'name': name + '-renamed'}) is None
    assert call('/settings/models/update', {'id': model_id}) is None
    assert call('/settings/models/' + model_id) == {**detail, 'name': name + '-renamed'}
finally:
    assert call('/settings/models/delete', {'id': model_id}) is None
assert call('/settings/models/' + model_id)['error']['code'] == 'MODEL_NOT_FOUND'
assert call('/settings/models/update', {'id': model_id})['error']['code'] == 'MODEL_NOT_FOUND'
print('PASS model create ID, empty update, public detail, missing model')
