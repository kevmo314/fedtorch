import plugin
import torch

def eval():
    a = torch.rand(100000000, device='cuda')
    b = torch.rand(100000000, device='cuda')
    return (a + b).sum()

# print(eval())

fl = plugin.FederatedLearning()
print(fl.submit(eval))