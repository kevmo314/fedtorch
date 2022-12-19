import asyncio
import io
import json
import uuid

import aiohttp
import torch
import time
from flask import Blueprint, request
from torch.distributed.launcher.api import launch_agent

tasks = []

offers = [
    # {"offer_id": "cf9f835d-7cb3-4652-8d78-8dc00281bba9", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
    # {"offer_id": "7eb93a2a-137d-43cd-9726-1a44af460fcb", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
    # {"offer_id": "dcfead48-8f10-4cbe-8362-2266bd3fe73b", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
    # {"offer_id": "13fefeac-4ccf-4e0d-b05e-4281a439cd56", "host": "localhost:8080", "load": 1500, "bid": 10, "expires": 1234567890},
]

bp = Blueprint("api", __name__, url_prefix="/api")

@bp.route("/submit", methods=["POST"])
def submit_route():
    """
    This is the entry point for the user to submit a job.
    """
    # deserialize the json body
    load = request.args.get("load")
    payload = request.data
    return str(submit(load, payload))

@bp.route("/response", methods=["GET"])
def response_route():
    """
    This is the entry point for the allocator to respond to a job.
    """
    # sleep for 15 seconds
    for i in range(500):
        time.sleep(2)
        done = False
        for t in tasks:
            if t["state"] == "active":
                t["state"] = "complete"
                done = True
                break
        if done:
            time.sleep(5)
            break
    a = torch.rand(100000000, device='cuda')
    b = torch.rand(100000000, device='cuda')
    out = io.BytesIO()
    torch.save((a + b).sum(), out)
    return out.getvalue()


@bp.route("/work", methods=["POST"])
def work_route():
    """
    This is the entry point for a worker to execute a job.
    """
    # deserialize the json body
    body = json.loads(request.data)
    if body is None:
        return "Invalid request", 400
    if "job_id" not in body:
        return "Missing job_id", 400
    if "config" not in body:
        return "Missing config", 400
    if "payload" not in body:
        return "Missing payload", 400

    return json.dumps(work(body["job_id"], body["config"], body["payload"]))

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
