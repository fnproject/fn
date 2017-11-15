#include "osxkeychain_darwin.h"
#include <CoreFoundation/CoreFoundation.h>
#include <Foundation/NSValue.h>
#include <stdio.h>
#include <string.h>

char *get_error(OSStatus status) {
  char *buf = malloc(128);
  CFStringRef str = SecCopyErrorMessageString(status, NULL);
  int success = CFStringGetCString(str, buf, 128, kCFStringEncodingUTF8);
  if (!success) {
    strncpy(buf, "Unknown error", 128);
  }
  return buf;
}

char *keychain_add(struct Server *server, char *label, char *username, char *secret) {
  SecKeychainItemRef item;

  OSStatus status = SecKeychainAddInternetPassword(
    NULL,
    strlen(server->host), server->host,
    0, NULL,
    strlen(username), username,
    strlen(server->path), server->path,
    server->port,
    server->proto,
    kSecAuthenticationTypeDefault,
    strlen(secret), secret,
    &item
  );

  if (status) {
    return get_error(status);
  }

  SecKeychainAttribute attribute;
  SecKeychainAttributeList attrs;
  attribute.tag = kSecLabelItemAttr;
  attribute.data = label;
  attribute.length = strlen(label);
  attrs.count = 1;
  attrs.attr = &attribute;

  status = SecKeychainItemModifyContent(item, &attrs, 0, NULL);

  if (status) {
    return get_error(status);
  }

  return NULL;
}

char *keychain_get(struct Server *server, unsigned int *username_l, char **username, unsigned int *secret_l, char **secret) {
  char *tmp;
  SecKeychainItemRef item;

  OSStatus status = SecKeychainFindInternetPassword(
    NULL,
    strlen(server->host), server->host,
    0, NULL,
    0, NULL,
    strlen(server->path), server->path,
    server->port,
    server->proto,
    kSecAuthenticationTypeDefault,
    secret_l, (void **)&tmp,
    &item);

  if (status) {
    return get_error(status);
  }

  *secret = strdup(tmp);
  SecKeychainItemFreeContent(NULL, tmp);

  SecKeychainAttributeList list;
  SecKeychainAttribute attr;

  list.count = 1;
  list.attr = &attr;
  attr.tag = kSecAccountItemAttr;

  status = SecKeychainItemCopyContent(item, NULL, &list, NULL, NULL);
  if (status) {
    return get_error(status);
  }

  *username = strdup(attr.data);
  *username_l = attr.length;
  SecKeychainItemFreeContent(&list, NULL);

  return NULL;
}

char *keychain_delete(struct Server *server) {
  SecKeychainItemRef item;

  OSStatus status = SecKeychainFindInternetPassword(
    NULL,
    strlen(server->host), server->host,
    0, NULL,
    0, NULL,
    strlen(server->path), server->path,
    server->port,
    server->proto,
    kSecAuthenticationTypeDefault,
    0, NULL,
    &item);

  if (status) {
    return get_error(status);
  }

  status = SecKeychainItemDelete(item);
  if (status) {
    return get_error(status);
  }
  return NULL;
}

char * CFStringToCharArr(CFStringRef aString) {
  if (aString == NULL) {
    return NULL;
  }
  CFIndex length = CFStringGetLength(aString);
  CFIndex maxSize =
  CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
  char *buffer = (char *)malloc(maxSize);
  if (CFStringGetCString(aString, buffer, maxSize,
                         kCFStringEncodingUTF8)) {
    return buffer;
  }
  return NULL;
}

char *keychain_list(char *credsLabel, char *** paths, char *** accts, unsigned int *list_l) {
    CFStringRef credsLabelCF = CFStringCreateWithCString(NULL, credsLabel, kCFStringEncodingUTF8);
    CFMutableDictionaryRef query = CFDictionaryCreateMutable (NULL, 1, NULL, NULL);
    CFDictionaryAddValue(query, kSecClass, kSecClassInternetPassword);
    CFDictionaryAddValue(query, kSecReturnAttributes, kCFBooleanTrue);
    CFDictionaryAddValue(query, kSecMatchLimit, kSecMatchLimitAll);
    CFDictionaryAddValue(query, kSecAttrLabel, credsLabelCF);
    //Use this query dictionary
    CFTypeRef result= NULL;
    OSStatus status = SecItemCopyMatching(
                                          query,
                                          &result);

    CFRelease(credsLabelCF);

    //Ran a search and store the results in result
    if (status) {
        return get_error(status);
    }
    CFIndex numKeys = CFArrayGetCount(result);
    *paths = (char **) malloc((int)sizeof(char *)*numKeys);
    *accts = (char **) malloc((int)sizeof(char *)*numKeys);
    //result is of type CFArray
    for(CFIndex i=0; i<numKeys; i++) {
        CFDictionaryRef currKey = CFArrayGetValueAtIndex(result,i);

        CFStringRef protocolTmp = CFDictionaryGetValue(currKey, CFSTR("ptcl"));
        if (protocolTmp != NULL) {
            CFStringRef protocolStr = CFStringCreateWithFormat(NULL, NULL, CFSTR("%@"), protocolTmp);
            if (CFStringCompare(protocolStr, CFSTR("htps"), 0) == kCFCompareEqualTo) {
                protocolTmp = CFSTR("https://");
            }
            else {
                protocolTmp = CFSTR("http://");
            }
            CFRelease(protocolStr);
        }
        else {
            char * path = "0";
            char * acct = "0";
            (*paths)[i] = (char *) malloc(sizeof(char)*(strlen(path)));
            memcpy((*paths)[i], path, sizeof(char)*(strlen(path)));
            (*accts)[i] = (char *) malloc(sizeof(char)*(strlen(acct)));
            memcpy((*accts)[i], acct, sizeof(char)*(strlen(acct)));
            continue;
        }
        
        CFMutableStringRef str = CFStringCreateMutableCopy(NULL, 0, protocolTmp);
        CFStringRef serverTmp = CFDictionaryGetValue(currKey, CFSTR("srvr"));
        if (serverTmp != NULL) {
            CFStringAppend(str, serverTmp);
        }
        
        CFStringRef pathTmp = CFDictionaryGetValue(currKey, CFSTR("path"));
        if (pathTmp != NULL) {
            CFStringAppend(str, pathTmp);
        }
        
        const NSNumber * portTmp = CFDictionaryGetValue(currKey, CFSTR("port"));
        if (portTmp != NULL && portTmp.integerValue != 0) {
            CFStringRef portStr = CFStringCreateWithFormat(NULL, NULL, CFSTR("%@"), portTmp);
            CFStringAppend(str, CFSTR(":"));
            CFStringAppend(str, portStr);
            CFRelease(portStr);
        }
        
        CFStringRef acctTmp = CFDictionaryGetValue(currKey, CFSTR("acct"));
        if (acctTmp == NULL) {
            acctTmp = CFSTR("account not defined");
        }

        char * path = CFStringToCharArr(str);
        char * acct = CFStringToCharArr(acctTmp);

        //We now have all we need, username and servername. Now export this to .go
        (*paths)[i] = (char *) malloc(sizeof(char)*(strlen(path)+1));
        memcpy((*paths)[i], path, sizeof(char)*(strlen(path)+1));
        (*accts)[i] = (char *) malloc(sizeof(char)*(strlen(acct)+1));
        memcpy((*accts)[i], acct, sizeof(char)*(strlen(acct)+1));

        CFRelease(str);
    }
    *list_l = (int)numKeys;
    return NULL;
}

void freeListData(char *** data, unsigned int length) {
     for(int i=0; i<length; i++) {
        free((*data)[i]);
     }
     free(*data);
}
