.. _amctl_config_subpath:

amctl config subpath
--------------------

Update client asset loading to use the configured subpath

Synopsis
~~~~~~~~


Update the hard-coded production client asset paths to take into account Antimatter running on a subpath. This command needs access to the Antimatter assets directory to be able to rewrite the paths.

::

  amctl config subpath [flags]

Examples
~~~~~~~~

::

    # you can rewrite the assets to use a subpath
    amctl config subpath --assets-dir /opt/antimatter/client --path /antimatter

    # the subpath can have multiple steps
    amctl config subpath --assets-dir /opt/antimatter/client --path /my/custom/subpath

    # or you can fallback to the root path passing /
    amctl config subpath --assets-dir /opt/antimatter/client --path /

Options
~~~~~~~

::

  -a, --assets-dir string   directory of the Antimatter assets in the local filesystem
  -h, --help                help for subpath
  -p, --path string         path to update the assets with

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

* `amctl config <amctl_config.rst>`_ 	 - Configuration

