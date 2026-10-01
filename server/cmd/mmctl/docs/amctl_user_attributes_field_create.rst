.. _amctl_user_attributes_field_create:

amctl user attributes field create
----------------------------------

Create a User Attributes field

Synopsis
~~~~~~~~


Create a new User Attributes field with the specified name and type.

::

  amctl user attributes field create [name] [type] [flags]

Examples
~~~~~~~~

::

    user attributes field create "Department" text --managed
    user attributes field create "Skills" multiselect --option Go --option React --option Python
    user attributes field create "Level" select --attrs '{"visibility":"always"}'

Options
~~~~~~~

::

      --attrs string     Full attrs JSON object for advanced configuration
  -h, --help             help for create
      --managed          Mark field as admin-managed (overrides --attrs)
      --option strings   Add an option for select/multiselect fields (overrides --attrs, can be repeated)

Options inherited from parent commands
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

::

      --config string                path to the configuration file (default "$XDG_CONFIG_HOME/amctl/config")
      --disable-pager                disables paged output
      --insecure-sha1-intermediate   allows to use insecure TLS protocols, such as SHA-1
      --insecure-tls-version         allows to use TLS versions 1.0 and 1.1
      --json                         the output format will be in json format
      --local                        allows communicating with the server through a unix socket
      --quiet                        prevent amctl to generate output for the commands
      --strict                       will only run commands if the amctl version matches the server one
      --suppress-warnings            disables printing warning messages

SEE ALSO
~~~~~~~~

* `amctl user attributes field <amctl_user_attributes_field.rst>`_ 	 - Management of User Attributes fields

