from flask import Flask, request, render_template, redirect, Response
import asyncio
import json
from absl import flags
from absl import app as absl_app
import requests
import uuid

import sys
import api
import federated

import time
import datetime
import torch
import io

_PEERS = flags.DEFINE_multi_string('peers', None, 'list of upstream instances, e.g. "8.8.8.8:5000')
_PORT = flags.DEFINE_integer('port', 5000, 'serving port')

app = Flask(__name__)

@app.route("/")
def index():
    return render_template("index.html",
        pending_tasks=[t for t in api.tasks if t["state"] == "pending"],
        active_tasks=[t for t in api.tasks if t["state"] == "active"],
        offers=api.offers,
    )

@app.route("/pubsub/join", methods=["POST"])
def pubsub_join():
    """
    Add a remote client to the local neighbors copy. Report back to the caller
    the current list of neighbors.

    Payload:
        {
            "user": "remote-uuid",
            "port": "5000",
        }

    Returns:
        {
            "user": "local-uuid",
            "neighbors": [
                {
                    "user": "neighbor-uuid",
                    "host": "8.8.8.8:5000",
                },
            ],
        }
    """
    # Add requestor to list of neighbors.
    neighbors = federated.merge([
        {
            "user": request.get_json()["user"],
            "host": f'{request.scheme}://{request.remote_addr}:{request.get_json()["port"]}',
        },
    ])

    # The UUID of the local instance is always the first item in the neighbor
    # list.
    uid = neighbors[0]["user"]
    return {
        "user": uid,
        "neighbors": neighbors[1:],
    }

@app.route("/pubsub/probe", methods=["POST"])
def pubsub_probe():
    """
    Attempts to reserve GPU downstream. The upstream caller cannot fulfill the request locally.

    Payload:
        {
            "task_id": "some-task-id",
            "target": "target-id",
        }

    Returns:
        {} or {
            "id": 1,
            "task_id": "some-task-id",
            "expiration": datetime.strftime(...),
        }
    """

    try:
        return federated.reserve({
            "target_id": request.get_json()["target_id"],
            "id": -1,
            "task_id": request.get_json()["task_id"],
            "lease": datetime.timedelta(seconds=60),
        })
    except federated.ServerGone as e:
        return Response(f"requested server is gone: {e}", status=409)

@app.route("/pubsub/extend", methods=["POST"])
def pubsub_extend():
    """
    Reserves GPU which was previously reserved via /pubsub/probe. The input ID must refer to the local device.

    Payload:
        {
            "id": 1,
            "task_id": "some-task-id",
            "lease": 3600,
        }

    Returns:
        {} or {
            "id": 1,
            "task_id": "some-task-id",
            "expiration": datetime.strftime(...),
        }
    """
    try:
        return federated.reserve({
            "target_id": request.get_json()["target_id"],
            "id": request.get_json()["id"],
            "task_id": request.get_json()["task_id"],
            "lease": datetime.timedelta(seconds=request.get_json()["lease"]),
        })
    except federated.ServerGone:
        return Response("requested server is gone", status=409)

@app.route("/tasks/<task_id>/approve")
def approve_task(task_id):
    # update the task to active
    for t in api.tasks:
        if str(t["task_id"]) == task_id:
            t["state"] = "active"
    # TODO: submit the task to the allocator
    # asyncio.ensure_future(api.approve(task_id))
    # redirect to index
    return redirect("/")

@app.route("/api/submit", methods=["POST"])
def route():
    """
    This is the entry point for the user to submit a job.
    """
    # deserialize the json body
    load = request.args.get("load")
    payload = request.data
    return str(api.submit(load, payload))

@app.route("/api/response", methods=["GET"])
def response():
    """
    This is the entry point for the allocator to respond to a job.
    """
    # sleep for 15 seconds
    for i in range(500):
        time.sleep(2)
        done = False
        for t in api.tasks:
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


@app.route("/api/work", methods=["POST"])
def work():
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

    return json.dumps(api.work(body["job_id"], body["config"], body["payload"]))

def link():
    """
    Links app with other instances.

    N.B.: This will have a race condition, since there is some amount of time
    between this set of requests are processed and when this server is actually
    up.
    """
    peers = _PEERS.value
    if peers is None:
        peers = []

    # Add self.
    uid = str(uuid.uuid4())

    federated.merge([
        {
            "user": uid,
            "host": f"http://127.0.0.1:{_PORT.value}",
        },
    ])

    for p in peers:
        resp = requests.post(f'{p}/pubsub/join', json = {
            "user": uid,
            "port": _PORT.value,
        })
        if resp.status_code == requests.codes.ok:
            data = resp.json()
            federated.merge([{
                "user": data["user"],
                "host": p,
            }] + data["neighbors"])

if __name__ == '__main__':
    absl_app.parse_flags_with_usage(sys.argv)
    link()  # This is being called twice due to server restarts.

    app.run(host='localhost', debug=True, port=_PORT.value)
