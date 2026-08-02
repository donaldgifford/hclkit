doctype "rfc" {
  id_prefix = "RFC"
  decides   = ["nope"]
}

doctype "memo" {
  id_prefix = "RFC"
}

doctype "unlabeled" "extra" {
  id_prefix = 42
}

doctype "bare" {}

settings {
  max_open = "ten"
}

mystery {}
