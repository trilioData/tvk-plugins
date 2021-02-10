#!/usr/bin/env python

"""
https://gist.github.com/clippit/4388272
"""
import argparse
import base64
import os
from datetime import datetime
from datetime import timedelta
from hashlib import sha1
import zlib
import uuid
from M2Crypto import DSA


def keymaker(organization, kube_uid, kubescope, server_id, license_edition, license_type_name,
             purchase_date=datetime.today(), licensed_for='10', evaluation='False', private_key='triliodata',
             expiration_date=datetime.today()+timedelta(180)):
    license_types = ('ACADEMIC', 'COMMERCIAL', 'COMMUNITY', 'DEMONSTRATION',
                     'DEVELOPER', 'NON_PROFIT', 'OPEN_SOURCE', 'PERSONAL',
                     'STARTER', 'HOSTED', 'TESTING', 'EULA', 'OEM')
    license_editions = ('FreeTrial', 'Basic', 'STANDARD', 'PROFESSIONAL', 'ENTERPRISE')
    if license_type_name not in license_types:
        raise ValueError('License Type Name must be one of the following '
                         'values:\n\t%s' % ', '.join(license_types))

    if license_edition not in license_editions:
        raise ValueError('License Edition must be one of the following '
                         'values:\n\t%s' % ', '.join(license_editions))

    header = datetime.today().ctime()
    purchasestr = (type(purchase_date) == type('str') and purchase_date ) or purchase_date.strftime('%Y-%m-%d')
    expirationstr = (type(expiration_date) == type('str') and expiration_date ) or expiration_date.strftime('%Y-%m-%d')

    properties = {
        'CreationDate           ': purchasestr,
        'Edition                ': license_edition,
        'active                 ': 'true',
        'licenseVersion         ': '2',
        'MaintenanceExpiryDate  ': expirationstr,
        'Company                ': organization,
        'NumberOfUsers          ': '-1',
        'ServerID               ': server_id,
        'SEN                    ': 'SEN-' + str(uuid.uuid1()),
        'LicenseID              ': 'TVAULT-' + str(uuid.uuid1()),
        'Scope                  ': kubescope,
        'KubeUID                ': kube_uid,
        'Expiration             ': expirationstr,
        'PurchaseDate           ': purchasestr,
        'Capacity               ': licensed_for + ' Kube Nodes',
    }

    properties_text = '#%s\n%s' % \
                      (header, '\n'.join(['%s=%s' % (key, value) for key, value \
                                          in properties.items()]))
    properties_text = properties_text.encode(encoding='utf-8', errors='strict')
    compressed_properties_text = zlib.compress(properties_text, 9)
    license_text_prefix = map(chr, (13, 14, 12, 10, 15))
    license_text = "".join([c for c in map(chr, (13, 14, 12, 10, 15))]).encode(encoding='utf-8', errors='strict') + compressed_properties_text

    cwd = os.path.dirname(os.path.realpath(__file__))
    project_root = os.path.dirname(os.path.dirname(cwd))
    key_path = os.path.join(project_root, 'triliodata')

    dsa = DSA.load_key(key_path)
    assert dsa.check_key()
    license_signature = dsa.sign_asn1(sha1(license_text).digest())
    license_pair_base64_bytes = chr(len(license_text)).encode('UTF-8') + \
                                license_text + license_signature
    license_pair_base64 = base64.b64encode(license_pair_base64_bytes)
    license_str = '%sX02%s' % (license_pair_base64.decode("utf-8"),
                               base_n(len(license_pair_base64), 31))
    return license_str, properties


def main():

    arg_parser = argparse.ArgumentParser()
    arg_parser.add_argument('--organization', default='Redhat')
    arg_parser.add_argument('--server_id', default=str(uuid.uuid1()))
    arg_parser.add_argument('--kube_uid', default=str(uuid.uuid1()))
    arg_parser.add_argument('--kube_scope', default='namespaced')
    arg_parser.add_argument('--license_edition', default='ENTERPRISE')
    arg_parser.add_argument('--license_type_name', default='EULA')
    arg_parser.add_argument('--purchase_date', default=datetime.today())
    arg_parser.add_argument('--expiration_date', default=datetime.today()+timedelta(365 * 3 + 8 + 31))
    arg_parser.add_argument('--licensed_for', default='10')

    args = arg_parser.parse_args()

    lickey, licprops = keymaker(args.organization, args.kube_uid, args.kube_scope, args.server_id, args.license_edition,
                                args.license_type_name, purchase_date=args.purchase_date,
                                licensed_for=args.licensed_for,expiration_date=args.expiration_date)
    print(lickey)


def base_n(num, b, numerals="0123456789abcdefghijklmnopqrstuvwxyz"):
    return ((num == 0) and "0") or (base_n(num // b, b).lstrip("0") + numerals[num % b])


if __name__ == '__main__':
    main()
