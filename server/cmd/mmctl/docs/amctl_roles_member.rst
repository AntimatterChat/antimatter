.. _amctl_roles_member:

amctl roles member
------------------

Remove system admin privileges

Synopsis
~~~~~~~~


Remove system admin privileges from some users.

::

  amctl roles member [users] [flags]

Examples
~~~~~~~~

::

    # You can remove admin privileges from one user
    $ amctl roles member john_doe

    # Or demote multiple users at the same time
    $ amctl roles member john_doe jane_doe

Options
~~~~~~~

::

  -h, --help   help for member

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

* `amctl roles <amctl_roles.rst>`_ 	 - Manage user roles

