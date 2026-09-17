// Wires up the three splitter handles (input/code, result-editor/result-frame
// and the horizontal one between the input/code row and the result rows) so
// the user can drag them to resize the adjacent panes. Ace editors need an
// explicit resize() call after their container changes size, since they
// cache their own layout and don't watch for CSS changes on their own.

(function () {

  function resizeEditors() {
    if (window.inputEditor) inputEditor.resize();
    if (window.codeEditor) codeEditor.resize();
    if (window.resultEditor) resultEditor.resize();
  }

  // Makes a vertical resizer drag the flex-basis of the pane before it,
  // keeping the pane after it filling the remaining space in the row.
  function makeVerticalResizer(resizer, leftPane, rightPane) {
    if (!resizer || !leftPane || !rightPane) return;

    let dragging = false;
    let startX = 0;
    let startLeftWidth = 0;
    let rowWidth = 0;

    function onPointerMove(evt) {
      if (!dragging) return;
      var delta = evt.clientX - startX;
      var minWidth = 40;
      var newLeftWidth = Math.min(Math.max(startLeftWidth + delta, minWidth), rowWidth - minWidth);
      var leftBasisPercent = (newLeftWidth / rowWidth) * 100;
      leftPane.style.flex = '0 0 ' + leftBasisPercent + '%';
      rightPane.style.flex = '1 1 auto';
      resizeEditors();
    }

    function onPointerUp() {
      if (!dragging) return;
      dragging = false;
      resizer.classList.remove('resizing');
      document.body.classList.remove('resizing-col');
      document.removeEventListener('pointermove', onPointerMove);
      document.removeEventListener('pointerup', onPointerUp);
      if (resizer.hasPointerCapture && resizer.hasPointerCapture(activePointerId)) {
        resizer.releasePointerCapture(activePointerId);
      }
      resizeEditors();
    }

    var activePointerId = null;

    resizer.addEventListener('pointerdown', function (evt) {
      dragging = true;
      activePointerId = evt.pointerId;
      startX = evt.clientX;
      startLeftWidth = leftPane.getBoundingClientRect().width;
      rowWidth = resizer.parentElement.getBoundingClientRect().width;
      resizer.classList.add('resizing');
      document.body.classList.add('resizing-col');
      // Capturing the pointer on the resizer itself keeps pointermove/up
      // events routed here even when the cursor passes over an <iframe> or
      // the Ace editor, both of which otherwise swallow the events before
      // they bubble to the document-level listeners below.
      if (resizer.setPointerCapture) {
        resizer.setPointerCapture(evt.pointerId);
      }
      document.addEventListener('pointermove', onPointerMove);
      document.addEventListener('pointerup', onPointerUp);
      evt.preventDefault();
    });
  }

  // Makes the horizontal resizer drag the flex-basis of the pane above it
  // (the input/code row), leaving the group below it (button bar + result
  // row) to fill the rest of the available height.
  function makeHorizontalResizer(resizer, topPane, bottomPane) {
    if (!resizer || !topPane || !bottomPane) return;

    let dragging = false;
    let startY = 0;
    let startTopHeight = 0;
    let containerHeight = 0;

    function onPointerMove(evt) {
      if (!dragging) return;
      var delta = evt.clientY - startY;
      var minHeight = 40;
      var newTopHeight = Math.min(Math.max(startTopHeight + delta, minHeight), containerHeight - minHeight);
      var topBasisPercent = (newTopHeight / containerHeight) * 100;
      topPane.style.flex = '0 0 ' + topBasisPercent + '%';
      bottomPane.style.flex = '1 1 auto';
      resizeEditors();
    }

    function onPointerUp() {
      if (!dragging) return;
      dragging = false;
      resizer.classList.remove('resizing');
      document.body.classList.remove('resizing-row');
      document.removeEventListener('pointermove', onPointerMove);
      document.removeEventListener('pointerup', onPointerUp);
      if (resizer.hasPointerCapture && resizer.hasPointerCapture(activePointerId)) {
        resizer.releasePointerCapture(activePointerId);
      }
      resizeEditors();
    }

    var activePointerId = null;

    resizer.addEventListener('pointerdown', function (evt) {
      dragging = true;
      activePointerId = evt.pointerId;
      startY = evt.clientY;
      startTopHeight = topPane.getBoundingClientRect().height;
      containerHeight = resizer.parentElement.getBoundingClientRect().height;
      resizer.classList.add('resizing');
      document.body.classList.add('resizing-row');
      // See the comment in makeVerticalResizer: capturing the pointer keeps
      // events routed to the resizer even over iframes/Ace editors.
      if (resizer.setPointerCapture) {
        resizer.setPointerCapture(evt.pointerId);
      }
      document.addEventListener('pointermove', onPointerMove);
      document.addEventListener('pointerup', onPointerUp);
      evt.preventDefault();
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    makeVerticalResizer(
      document.getElementById('input-code-resizer'),
      document.getElementById('input-col'),
      document.getElementById('code-col')
    );

    makeVerticalResizer(
      document.getElementById('result-frame-resizer'),
      document.getElementById('result-editor-col'),
      document.getElementById('result-frame-container')
    );

    makeHorizontalResizer(
      document.getElementById('top-bottom-resizer'),
      document.getElementById('input-code-row'),
      document.getElementById('bottom-group')
    );

    window.addEventListener('resize', resizeEditors);
  });

})();
