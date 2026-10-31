declare namespace output = 'http://www.w3.org/2010/xslt-xquery-serialization';

import schema default element namespace "" at "schema-sample1.xsd";

declare option output:method 'xml';
declare option output:indent 'yes';

declare function local:process-node($node as node()) as node()* {
  typeswitch ($node) 
    case $n as document-node() return document { $n ! node() ! local:process-node(.) }
    case $n as element(value) return (comment { 'data($node) instance of xs:decimal:', data($node) instance of xs:decimal }, $n )
    case $n as element() return element { node-name($n) } { $n!@*, $n ! node() ! local:process-node(.) }
    default $n return $n
};

local:process-node(.)