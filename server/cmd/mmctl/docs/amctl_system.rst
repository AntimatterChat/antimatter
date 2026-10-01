.. _amctl_system:

amctl system
------------

System management

Synopsis
~~~~~~~~


System management commands for interacting with the server state and configuration.

Options
~~~~~~~

::

  -h, --help   help for system

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

* `amctl <amctl.rst>`_ 	 - Remote client for the Open Source, self-hosted Slack-alternative
* `amctl system clearbusy <amctl_system_clearbusy.rst>`_ 	 - Clears the busy state
* `amctl system getbusy <amctl_system_getbusy.rst>`_ 	 - Get the current busy state
* `amctl system nuke <amctl_system_nuke.rst>`_ 	 - Destructive operations that permanently delete data
* `amctl system setbusy <amctl_system_setbusy.rst>`_ 	 - Set the busy state to true
* `amctl system status <amctl_system_status.rst>`_ 	 - Prints the status of the server
* `amctl system supportpacket <amctl_system_supportpacket.rst>`_ 	 - Download a Support Packet
* `amctl system version <amctl_system_version.rst>`_ 	 - Prints the remote server version

