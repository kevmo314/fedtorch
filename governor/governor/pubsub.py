from flask import Blueprint, request

from . import federated

bp = Blueprint("pubsub", __name__, "/pubsub")

@bp.route("/join", methods=["POST"])
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

@bp.route("/probe", methods=["POST"])
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

@bp.route("/extend", methods=["POST"])
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
