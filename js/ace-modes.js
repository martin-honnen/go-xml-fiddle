// Keyed by the serialization method a result was written with, as well as by
// the code types the editors hold. 'adaptive' is a method too: it renders a
// sequence rather than a document, so it has no markup to highlight.
var modes = {
    'xml': 'ace/mode/xml',
    'html': 'ace/mode/html',
    'xhtml': 'ace/mode/xml',
    'json': 'ace/mode/json',
    'xsd': 'ace/mode/xml',
    'text': 'ace/mode/text',
    'adaptive': 'ace/mode/text',
    'xquery': 'ace/mode/xquery',
    'xpath' : 'ace/mode/xpath'
};

var filetypes = {
    '.htm': 'html',
    '.html': 'html',
    '.xml': 'xml',
    '.xsd': 'xml',
    '.xsl': 'xml',
    '.xslt': 'xml',
    '.xhtml' : 'xml',
    '.xquery' : 'xquery',
    '.xq' : 'xquery',
    '.xpath' : 'xpath',
    '.xp' : 'xpath',
    '.xpath31' : 'xpath',
    '.json' : 'json'
};

// The file name a result is offered for download under, and the media type of
// its blob, by serialization method. XHTML is saved as .xhtml so that a
// browser opening it parses it as XML, which is what it was written as.
var methodExtensions = {
    'xml': 'xml',
    'html': 'html',
    'xhtml': 'xhtml',
    'json': 'json',
    'text': 'txt',
    'adaptive': 'txt'
};

var methodMediaTypes = {
    'xml': 'application/xml',
    'html': 'text/html',
    'xhtml': 'application/xhtml+xml',
    'json': 'application/json',
    'text': 'text/plain',
    'adaptive': 'text/plain'
};

function setDocument(editor, content, mode) {
    if (mode && modes[mode]) {
        editor.session.setMode(modes[mode]);
        // Wrapping is for what has no structure to follow: the plain-text
        // methods, and the message and warning panes that come through as
        // text. Asking the mode rather than the name covers 'adaptive' too.
        editor.session.setUseWrapMode(modes[mode] === 'ace/mode/text');
    }
    editor.session.setValue(content);
}
