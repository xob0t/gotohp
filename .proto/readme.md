# Generating .proto files

1. Using python bbbp package, generate .proto files from message bytes.

```python
# pip install bbpb
import blackboxprotobuf

protobuf_hex = "hex here"
message_name = "HumanFriendlyMessageName"

protobuf_bytes = bytes.fromhex(protobuf_hex)
decoded_data, message_type = blackboxprotobuf.decode_message(protobuf_bytes)

blackboxprotobuf.export_protofile({message_name: message_type}, f"{message_name}.proto")
```

2. Using protoc generagte go code from .proto file

```bash
protoc --proto_path=. --go_out=. --go_opt=M.proto/GetUploadToken.proto=/generated proto/GetUploadToken.proto
```