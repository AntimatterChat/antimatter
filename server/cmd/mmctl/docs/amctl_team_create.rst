.. _amctl_team_create:

amctl team create
-----------------

Create a team

Synopsis
~~~~~~~~


Create a team.

::

  amctl team create [flags]

Examples
~~~~~~~~

::

    team create --name mynewteam --display-name "My New Team"
    team create --name private --display-name "My New Private Team" --private

Options
~~~~~~~

::

      --display-name string   Team Display Name
      --email string          Administrator Email (anyone with this email is automatically a team admin)
  -h, --help                  help for create
      --name string           Team Name
      --private               Create a private team.

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

* `amctl team <amctl_team.rst>`_ 	 - Management of teams

