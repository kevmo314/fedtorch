import sys
import time
import xmlrpc
from xmlrpc.server import SimpleXMLRPCServer
import torch
from torch.distributed.laucher.api import launch_agent
import io

server = SimpleXMLRPCServer(("127.0.0.1", int("5000")))

def submit(fn, serialized_args):
    # TODO: send fn, args to allocator and get allocations
    allocations = []

    for alloc in allocations:
        # TODO: generate a config
        config = None
        # connect to the remote allocation
        
        client = xmlrpc.client.ServerProxy(alloc)
        
        res = client.work(config, fn, serialized_args)

    return 0

def work(config, fn, serialized_args):
    args = torch.load(io.BytesIO(serialized_args))
    return launch_agent(config, fn, args)

server.register_function(submit)
server.register_function(work)
server.serve_forever()