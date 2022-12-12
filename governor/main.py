from flask import Flask, request, render_template, redirect
import asyncio
import json
from . import api
import time
import torch
import io

app = Flask(__name__)

@app.route("/")
def index():
    return render_template("index.html",
        pending_tasks=[t for t in api.tasks if t["state"] == "pending"],
        active_tasks=[t for t in api.tasks if t["state"] == "active"],
        offers=api.offers,
    )

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
