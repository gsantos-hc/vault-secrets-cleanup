[
  .[]
  | select(
      .type == "response"
      and .request.mount_type == "kv"
      and (.request.operation | IN("create", "read", "update", "delete"))
    )
  | {
      "namespace_path": .request.namespace.path // "",
      "mount_accessor": .request.mount_accessor,
      "mount_path": .request.mount_point,
      "secret_path": .request.path,
      "access_type": .request.operation // "",
      "timestamp": .time
    }
]