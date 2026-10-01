.. _amctl_bot_update:

amctl bot update
----------------

Update bot

Synopsis
~~~~~~~~


Update bot information.

::

  amctl bot update [username] [flags]

Examples
~~~~~~~~

::

    bot update testbot --username newbotusername

Options
~~~~~~~

::

      --description string    Optional. The new description text for the bot.
      --display-name string   Optional. The new display name for the bot.
  -h, --help                  help for update
      --username string       Optional. The new username for the bot.

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

* `amctl bot <amctl_bot.rst>`_ 	 - Management of bots

