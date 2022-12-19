import uuid

import requests
from flask import Flask, Response, redirect, render_template, request

from . import api
from . import pubsub
from . import federated

def create_app(test_config=None):
    app = Flask(__name__)

    app.register_blueprint(api.bp)
    app.register_blueprint(pubsub.bp)

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

    federated.link()

    return app

def main():
    app = create_app()
    app.run(host="0.0.0.0", port=5000)