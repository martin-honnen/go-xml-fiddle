document.addEventListener('DOMContentLoaded',
    function () {
        autoEvaluation = document.getElementById('auto-evaluate').checked;
        document.getElementById('input-types').addEventListener('click',
            function (evt) {
                var inputType = evt.currentTarget.form['input-type'].value;
                if (inputType !== 'None') {
                    inputEditor.session.setMode(modes[inputType.toLowerCase()]);
                }
                document.getElementById('input-col').style.display =
                    inputType === 'None' ? 'none' : '';
                return true;
            },
            false
        );
        document.getElementById('code-types').addEventListener('click',
            function (evt) {
                var codeType = evt.currentTarget.form['code-type'].value;
                if (codeType !== undefined) {
                    codeEditor.session.setMode(modes[codeType.toLowerCase()]);
                }
                return true;
            },
            false
        );
        document.getElementById('render-result').addEventListener('click',
            function (evt) {
                var showFrame = document.getElementById('render-box').checked;
                var resultEditorCol = document.getElementById('result-editor-col');
                var resultFrameResizer = document.getElementById('result-frame-resizer');

                document.getElementById('result-frame-container').style.display = showFrame ? '' : 'none';
                resultFrameResizer.style.display = showFrame ? '' : 'none';

                if (showFrame) {
                    // Restore whatever split the user had dragged to before hiding the frame.
                    resultEditorCol.style.flex = resultEditorCol.dataset.savedFlex || '';
                    delete resultEditorCol.dataset.savedFlex;
                } else {
                    // Remember the current split (may have been set by dragging the resizer)
                    // and let the editor take the full row while the frame is hidden.
                    resultEditorCol.dataset.savedFlex = resultEditorCol.style.flex || '';
                    resultEditorCol.style.flex = '1 1 100%';
                }

                if (window.resultEditor) resultEditor.resize();

                return true;
            },
            false
        );
		document.getElementById('auto-evaluate').addEventListener('click',
            function (evt) {
                autoEvaluation = evt.target.checked;
            },
            false
        );
    },
    false
)