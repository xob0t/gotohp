import sys
import ast
import os
import blackboxprotobuf

# Add scratch path to sys.path
scratch_dir = r"C:\Users\AcPro7\Documents\gotohp"
sys.path.insert(0, r"C:\Users\AcPro7\.gemini\antigravity-ide\brain\681bb13a-46d8-44bf-9e17-f063093ffe01\scratch")
import message_types

# Read api.py and parse AST
api_path = r"C:\Users\AcPro7\.gemini\antigravity-ide\brain\681bb13a-46d8-44bf-9e17-f063093ffe01\scratch\api.py"
with open(api_path, "r", encoding="utf-8") as f:
    api_source = f.read()

tree = ast.parse(api_source)

# Helper to find proto_body in a function
def get_proto_body_dict(func_name):
    for node in ast.walk(tree):
        if isinstance(node, ast.FunctionDef) and node.name == func_name:
            for subnode in ast.walk(node):
                if isinstance(subnode, ast.Assign):
                    for target in subnode.targets:
                        if isinstance(target, ast.Name) and target.id == "proto_body":
                            # Evaluate the dictionary node using literal_eval
                            source_segment = ast.get_source_segment(api_source, subnode.value)
                            # Replace variables in the source segment
                            source_segment = source_segment.replace("sync_token", "''")
                            source_segment = source_segment.replace("resume_token", "''")
                            source_segment = source_segment.replace("self.android_api_version", "28")
                            source_segment = source_segment.replace("self.client_version_code", "49029607")
                            # Safely evaluate
                            d = ast.literal_eval(source_segment)
                            return d
    return None

proto_body_state = get_proto_body_dict("get_library_state")
proto_body_page_init = get_proto_body_dict("get_library_page_init")
proto_body_page = get_proto_body_dict("get_library_page")

if not proto_body_state or not proto_body_page_init or not proto_body_page:
    print("Error: Could not extract proto bodies")
    sys.exit(1)

# Helper to recursively convert string digit keys to int
def keys_to_int(d):
    if isinstance(d, dict):
        new_d = {}
        for k, v in d.items():
            if isinstance(k, str) and k.isdigit():
                new_key = int(k)
            else:
                new_key = k
            new_d[new_key] = keys_to_int(v)
        return new_d
    elif isinstance(d, list):
        return [keys_to_int(x) for x in d]
    else:
        return d

# Helper to recursively map 'string' type to 'bytes' in typedefs
def map_typedef_types(typedef):
    if isinstance(typedef, dict):
        new_t = {}
        for k, v in typedef.items():
            if k == "type" and v == "string":
                new_t[k] = "bytes"
            else:
                new_t[k] = map_typedef_types(v)
        return new_t
    elif isinstance(typedef, list):
        return [map_typedef_types(x) for x in typedef]
    else:
        return typedef

# Function to format bytes as Go slice
def to_go_slice(b):
    return "[]byte{" + ", ".join(f"0x{x:02x}" for x in b) + "}"

# For GET_LIB_STATE:
# We want to serialize "1" (without "6" sync_token) and "2" separately.
root_state_dict = proto_body_state["1"].copy()
if "6" in root_state_dict:
    del root_state_dict["6"]

# Convert keys to int
root_state_dict_int = keys_to_int(root_state_dict)
root_state_typedef = map_typedef_types(message_types.GET_LIB_STATE["1"]["message_typedef"])
root_state_bytes = blackboxprotobuf.encode_message(root_state_dict_int, root_state_typedef)

# Serialize field "2" of GET_LIB_STATE
field2_state_dict_int = keys_to_int(proto_body_state["2"])
field2_state_typedef = map_typedef_types(message_types.GET_LIB_STATE["2"]["message_typedef"])
field2_state_bytes = blackboxprotobuf.encode_message(field2_state_dict_int, field2_state_typedef)

# For GET_LIB_PAGE_INIT:
root_page_init_dict = proto_body_page_init["1"].copy()
if "4" in root_page_init_dict:
    del root_page_init_dict["4"]

root_page_init_dict_int = keys_to_int(root_page_init_dict)
root_page_init_typedef = map_typedef_types(message_types.GET_LIB_PAGE_INIT["1"]["message_typedef"])
root_page_init_bytes = blackboxprotobuf.encode_message(root_page_init_dict_int, root_page_init_typedef)

field2_page_init_dict_int = keys_to_int(proto_body_page_init["2"])
field2_page_init_typedef = map_typedef_types(message_types.GET_LIB_PAGE_INIT["2"]["message_typedef"])
field2_page_init_bytes = blackboxprotobuf.encode_message(field2_page_init_dict_int, field2_page_init_typedef)

# For GET_LIB_PAGE:
root_page_dict = proto_body_page["1"].copy()
if "4" in root_page_dict:
    del root_page_dict["4"]
if "6" in root_page_dict:
    del root_page_dict["6"]

root_page_dict_int = keys_to_int(root_page_dict)
root_page_typedef = map_typedef_types(message_types.GET_LIB_PAGE["1"]["message_typedef"])
root_page_bytes = blackboxprotobuf.encode_message(root_page_dict_int, root_page_typedef)

field2_page_dict_int = keys_to_int(proto_body_page["2"])
field2_page_typedef = map_typedef_types(message_types.GET_LIB_PAGE["2"]["message_typedef"])
field2_page_bytes = blackboxprotobuf.encode_message(field2_page_dict_int, field2_page_typedef)

# Write output file directly in UTF-8
out_path = r"C:\Users\AcPro7\Documents\gotohp\backend\serialized_proto_templates.go"
with open(out_path, "w", encoding="utf-8") as out:
    out.write("package backend\n\n")
    out.write("// This file is auto-generated. DO NOT EDIT.\n\n")
    out.write("var (\n")
    out.write("    // GET_LIB_STATE templates\n")
    out.write(f"    libStateRootStatic = {to_go_slice(root_state_bytes)}\n")
    out.write(f"    libStateField2Static = {to_go_slice(field2_state_bytes)}\n\n")
    out.write("    // GET_LIB_PAGE_INIT templates\n")
    out.write(f"    libPageInitRootStatic = {to_go_slice(root_page_init_bytes)}\n")
    out.write(f"    libPageInitField2Static = {to_go_slice(field2_page_init_bytes)}\n\n")
    out.write("    // GET_LIB_PAGE templates\n")
    out.write(f"    libPageRootStatic = {to_go_slice(root_page_bytes)}\n")
    out.write(f"    libPageField2Static = {to_go_slice(field2_page_bytes)}\n")
    out.write(")\n")

print("Templates written to backend/serialized_proto_templates.go")
