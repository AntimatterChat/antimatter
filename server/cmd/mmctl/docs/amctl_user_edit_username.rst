.. _amctl_user_edit_username:

amctl user edit username
------------------------

Edit user's username

Synopsis
~~~~~~~~


Edit a user's username.

::

  amctl user edit username [user] [new username] [flags]

Examples
~~~~~~~~

::

  user edit username user@example.com newusername

Options
~~~~~~~

::

  -h, --help   help for username

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

* `amctl user edit <amctl_user_edit.rst>`_ 	 - Edit user properties

