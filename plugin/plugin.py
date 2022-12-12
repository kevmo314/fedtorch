import argparse
import pickle
import io
import torch
import requests

torch.cuda.set_per_process_memory_fraction(0.1, 0)

class FederatedLearning:
    def __init__(self, host = 'localhost:5000'):
        self.host = host

    def submit(self, fn, *args):
        """
        Submit a function to be executed on the remote worker.
        """
        serialized = io.BytesIO()
        torch.save({
            'fn': fn,
            'args': args,
        }, serialized)

        resp = requests.post(
            'http://' + self.host + '/api/submit?load=1000',
            data=serialized.getvalue(),
        )
        resp.raise_for_status()
        id = resp.text

        # print out a mesage
        print('Federated PyTorch ----------------------------')
        print('Submitted job with id: %s' % id)
        print('Open http://%s to see the status of the job.' % self.host)
        print()

        res = requests.get('http://' + self.host + '/api/response?id=' + id)
        res.raise_for_status()
        return torch.load(io.BytesIO(res.content))

