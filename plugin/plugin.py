import argparse
import pickle
import io
import torch
import xmlrpc

class FederatedLearning:
    def __init__(self, host = 'localhost:5000'):
        self.client = xmlrpc.client.ServerProxy(host)

    def submit(self, fn, *args):
        """
        Submit a function to be executed on the remote worker.
        """
        serialized = io.BytesIO()
        torch.save(args, serialized)

        return self.client.submit(fn, serialized.getvalue())