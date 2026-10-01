.. _amctl_user_edit:

amctl user edit
---------------

Edit user properties

Synopsis
~~~~~~~~


Edit user properties like username, email, or authdata.

Options
~~~~~~~

::

  -h, --help   help for edit

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

* `amctl user <amctl_user.rst>`_ 	 - Management of users
* `amctl user edit authdata <amctl_user_edit_authdata.rst>`_ 	 - Edit user's authdata
* `amctl user edit email <amctl_user_edit_email.rst>`_ 	 - Edit user's email
* `amctl user edit username <amctl_user_edit_username.rst>`_ 	 - Edit user's username

