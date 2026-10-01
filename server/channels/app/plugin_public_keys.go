// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

// Fingerprints of the hard-coded plugin signing keys below.
const (
	antimatterPluginPublicKeyFingerprint = "06C8103246770F08BBA778B9E69B05C392A69A45"
)

// builtinPluginPublicKeys are the plugin signing keys trusted without any configuration, tried in
// order. Admins can trust additional publishers through PluginSettings.SignaturePublicKeyFiles.
var builtinPluginPublicKeys = []pluginPublicKey{
	{name: "Antimatter", key: antimatterPluginPublicKey},
}

// pluginPublicKey is a named, armored or binary OpenPGP public key trusted to sign plugins.
type pluginPublicKey struct {
	name string
	key  []byte
}

// antimatterPluginPublicKey is the public half of the key the Antimatter build pipeline signs
// prepackaged plugins with.
var antimatterPluginPublicKey = []byte(`-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGq8BDwBEACo4xDqekUKgAl2oziJbJC8zigu69ZmU99c7X3x1BywpkMzjr8R
CGwk8ahJa0bnP/iT9t83iapx/8bRnX/hGSYHOEIF1JUkgUflDJH6NBqHha2HVvgn
PC81kpN/1VwzNip8QmrJn0oDi2aVxdv7rf32IfORktBUAi02KRlthIkFJDlId/Db
D7MyeLc7AVEDTdIQ9X9Okk1vKPhZSxUlp+esTmPMDf8DrXYWHE7DzY7mcr8GOLP8
PsvZ7LW2LdCiGbMlaDjZiaeKsUyXWb1MdX7d9r3TL0THfAPFBM4soBXeSAa5eHm3
wND82Ae9xH+/JmsVQOhnIfHrfqZHze727F2tHJnjOoYUaVaFDXSE95KV/OSH/Ywn
W9f3l20dLhaOnh9CXAQKxQqvubtXiw93ITe2dcJNzCIH8/zsz0kh+2961ASgZVAW
zXl5hYwUQ5LCyjXvdmW/3C9XAXMvzUvLuX8pkmMNG/kT2FLewqD5gFDKAymzl/U1
tbs2HdkGaFCAFrXwm5iaL4Xxu6hVfKQcyoQv0aiEdfNt3Z8sahkHsbrpB1V+4ZBT
dCtuhMMnej904RsvjvCmMHy7qQiJnRmq5fpbGyiF8u2IbBF8H168f7tQePhapY7E
kRVEQCNq+xt4/ltaSEjpQLrFV4QdHEYxnTldHmKanqlHrimpvJzTOaP+8QARAQAB
tBlBbnRpbWF0dGVyIFBsdWdpbiBTaWduaW5niQJOBBMBCgA4FiEEBsgQMkZ3Dwi7
p3i55psFw5KmmkUFAmq8BDwCGwMFCwkIBwIGFQoJCAsCBBYCAwECHgECF4AACgkQ
5psFw5KmmkXxtQ//YUIlV4+Z1i4O+T53s5QQC+f3/q+h5gCsnCf/a3uWLY6rR24+
1Jts9b1avzi3rm4/7DzL4KInZBS5Zm12vdznCNoZ3IWhpKPvliK/WAf8ddDn3+l8
MKuIBhItaVqXaRy6jVvi7+MzuJSxRl8F5LK/Krhnx6ewqpNXV2sszzkH00pqfV2Z
vAWJcv62Y7uojlMwKaSC783cy6F+ojVrciNEXAcibzPer2Z//J3cKG8Ds1R6X3Fy
o/ud6QQXTqrtFRifdzXfyX4HVYgsodPrzrgFuXEEpJpd0ayjZvRTFtPXiWBnKkB6
ohzDu1TFfUFcygdIdY3Sab8o+BEu6UBJeVhBpsxxJFOATJHSN0y8Tj+f+8HKuA39
trKl1c8A5lD9IL2DYBfYKCEDAhHySr/Bqvv4yFf7L/YBw6hYp31qHFGngOeaM0G0
TeCITK8sPIx/6FRCAmYp2way+qDCHiHxHC28eXSf1uaYERFsZ8Umfopw9Umlr0fi
FhcjqVaMYIv/1eK+V83ODs99wg9QJCcpoJ6yjg9ivF0awl6C+UqP8s4HSBAPq25y
oYWcE97tZ9lkXczjxIwaHg5YoqbHHO9MUx3qW0M2Khmdv1oUahjofvj71fk2rTGX
FnCZ1pcSh4Y7o8JU8uBycOk7ZtkMqMAskqeBkngCTcng0/0dXuve81uvWTE=
=VAo5
-----END PGP PUBLIC KEY BLOCK-----`)
