import sys
import time
from xmlrpc.server import SimpleXMLRPCServer
import torch
import io

server = SimpleXMLRPCServer(("127.0.0.1", int("5000")))

def submit(fn, serialized_args):
    args = torch.load(io.BytesIO(serialized_args))

    # TODO: send fn, args to allocator

    return 0

server.register_function(submit)
server.serve_forever()