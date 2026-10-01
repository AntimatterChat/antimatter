.. _amctl_team_archive:

amctl team archive
------------------

Archive teams

Synopsis
~~~~~~~~


Archive some teams.
Archives a team along with all related information including posts from the database.

::

  amctl team archive [teams] [flags]

Examples
~~~~~~~~

::

    team archive myteam

Options
~~~~~~~

::

      --confirm   Confirm you really want to archive the team and a DB backup has been performed.
  -h, --help      help for archive

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

