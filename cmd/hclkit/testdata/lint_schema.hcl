block "doctype" {
  labels = 1
}

block "settings" {}

attribute "id_prefix" {
  block    = "doctype"
  required = true
  type     = string
}

attribute "max_open" {
  block    = "settings"
  type     = number
}

reference {
  verb        = "decides"
  target_kind = "doctype"
}

unique {
  block_kind = "doctype"
  attribute  = "id_prefix"
}
