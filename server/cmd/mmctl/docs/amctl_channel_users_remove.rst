.. _amctl_channel_users_remove:

amctl channel users remove
--------------------------

Remove users from channel

Synopsis
~~~~~~~~


Remove some users from channel

::

  amctl channel users remove [channel] [users] [flags]

Examples
~~~~~~~~

::

    channel users remove myteam:mychannel user@example.com username
    channel users remove myteam:mychannel --all-users

Options
~~~~~~~

::

      --all-users   Remove all users from the indicated channel.
  -h, --help        help for remove

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

* `amctl channel users <amctl_channel_users.rst>`_ 	 - Management of channel users

