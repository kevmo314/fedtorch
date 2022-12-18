"""
TODO(minkezhang): Use ActivityPub for a more formal federated framework.
"""
import threading
import datetime

neighbors_lock = threading.Lock()
neighbors = [
    # First entry is always self.
    # {"user": "12345678-1234-5678-1234-567812345678", "host": "8.8.8.8:5000"},
]

gpus_lock = threading.Lock()
gpus = [
    {
        "id": 0,
        "task_id": "",
        "expiration": datetime.datetime.now(),
    },
    # {"id": 0, "task_id": ..., "expiration": datetime },
]

# Remote server should update its list of neighbors and remove from its local
# neighbor list, then retry with other neighbors.
class ServerGone(Exception):
    pass

def reserve(lease):
    now = datetime.datetime.now()
    v = {}

    # Make sure we are taking to the correct server instance, which may have
    # been restarted in the meantime.
    neighbors_lock.acquire()
    try:
        uid = neighbors[0]["user"]
    finally:
        neighbors_lock.release()

    if lease["target_id"] != uid:
        raise ServerGone(f'server {lease["target_id"]} has since shut down, please update records to use {uid} instead')

    gpus_lock.acquire()
    try:
        # Extend reservation. Only extend if task_id matches, otherwise fail.
        if lease["id"] >= 0:
            for gpu in gpus:
                if gpu["id"] == lease["id"] and gpu["task_id"] == lease["task_id"] and now < gpu["expiration"]:
                    gpu["expiration"] = now + lease["lease"]
                    v = dict(gpu)
        else:
            for gpu in gpus:
                if gpu["expiration"] < now:
                    id = gpu["id"]
                    gpu["task_id"] = lease["task_id"]
                    gpu["expiration"] = now + lease["lease"]

                    v = dict(gpu)
    finally:
        gpus_lock.release()

    return v

def get_neighbors():
    neighbors_lock.acquire()
    try:
        vs = [dict(x) for x in neighbors]
    finally:
        neighbors_lock.release()
    return vs

def merge(updates):
    neighbors_lock.acquire()
    vs = []
    try:
        uuids = set([x["user"] for x in neighbors])
        for n in updates:
            if n["user"] not in uuids:
                uuids.add(n["user"])
                neighbors.append(n)
        vs = [dict(x) for x in neighbors]
    finally:
        neighbors_lock.release()
    return vs


def drop(n):
    """
    Neighbor dropped offline.

    TODO(minkezhang): Worry about if graphs become disconnected.
    """

    neighbors_lock.acquire()
    neighbors.remove(n)
    neighbors_lock.remove()
