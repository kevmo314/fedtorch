import aiohttp
import asyncio
import io
import json
import torch
import uuid
from torch.distributed.launcher.api import launch_agent

tasks = []

offers = [
    # {"offer_id": "cf9f835d-7cb3-4652-8d78-8dc00281bba9", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
    # {"offer_id": "7eb93a2a-137d-43cd-9726-1a44af460fcb", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
    # {"offer_id": "dcfead48-8f10-4cbe-8362-2266bd3fe73b", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
    # {"offer_id": "13fefeac-4ccf-4e0d-b05e-4281a439cd56", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
]

def submit(load, payload):
    task_id = uuid.uuid4()
    tasks.append({
        'task_id': task_id,
        'state': 'pending',
        'load': load,
        'payload': payload,
    })
    return task_id


async def approve(task_id):
    task = tasks[task_id]
    load, payload = task['load'], task['payload']

    # set task state to active
    task['state'] = 'active'

    # TODO: send fn, args to allocator and get allocations
    allocations = []

    async with aiohttp.ClientSession() as session:
        responses = []
        for alloc in allocations:
            # TODO: generate a serializable config
            config = None
            resp = session.post(
                'http://%s/api/work' % alloc['host'],
                data=json.dumps({
                    'job_id': alloc['job_id'],
                    'config': config,
                    'payload': payload,
                }))
            responses.append(resp)
        return await asyncio.gather(*responses)

def work(job_id, config, payload: bytes):
    # TODO: validate job_id from the allocator and get the offer
    # that we sent associated with it.
    deserialized = torch.load(io.BytesIO(payload))
    fn = deserialized['fn']
    args = deserialized['args']
    return launch_agent(config, fn, args)
