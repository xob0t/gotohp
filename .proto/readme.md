# Generating .proto files

1. Generate .proto files from encoded message with bbbp python package

    ```python
    # pip install bbpb
    import blackboxprotobuf

    protobuf_hex = "hex here"
    message_name = "HumanFriendlyMessageName"

    protobuf_bytes = bytes.fromhex(protobuf_hex)
    decoded_data, message_type = blackboxprotobuf.decode_message(protobuf_bytes)

    blackboxprotobuf.export_protofile({message_name: message_type}, f"{message_name}.proto")
    ```

1. Generagte go code from .proto file with protoc

    ```bash
    protoc --proto_path=. --go_out=. --go_opt=M.proto/HumanFriendlyMessageName.proto=/generated proto/HumanFriendlyMessageName.proto
    ```
