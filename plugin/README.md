# Running on a GCE instance

## Machine shape

machine family: GPU
GPU Type: 1x NVIDIA T4
machine type: n1-highmem-2 (2 vCPU + 13GB RAM)
boot disk size: > 100GB
image: Debian 10 based Deep Learning VM with M100 (this is just the recommended image from GCE setup)

This will cost ~$200 - $300 / mo.

## packages

Need a version of `torch` with CUDA 11.1 support.

```bash
# https://stackoverflow.com/a/74394696/873865

pip3 install torch+cu111 torchvision torchaudio -f https://download.pytorch.org/whl/torch_stable.html
```

## Run

```bash
python3 example.py --no-mps
```

Should take around 3min or so for GPU utilization.
