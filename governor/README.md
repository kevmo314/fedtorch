# Governor

## Protobuf

```bash
protoc -I ./ --go_out=api/ --go_opt=paths=import --go_opt=module=github.com/kevmo314/fedtorch/governor/api api/*proto
```

## Development

### Local

```bash
poetry install
poetry run governor
```

### Docker

```bash
docker build -t fedtorch/governor -f dev.Dockerfile .
docker run -it --rm -p 5000:5000 -v $(pwd):/app --gpus all fedtorch/governor
```