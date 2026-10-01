.. _amctl_permissions_role_show:

amctl permissions role show
---------------------------

Show the role information

Synopsis
~~~~~~~~


Show all the information about a role.

::

  amctl permissions role show <role_name> [flags]

Examples
~~~~~~~~

::

    permissions role show system_user

Options
~~~~~~~

::

  -h, --help   help for show

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

* `amctl permissions role <amctl_permissions_role.rst>`_ 	 - Management of roles

