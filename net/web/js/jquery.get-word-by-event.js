/**
 * GetWordByEvent jQuery plugin
 * ============================
 * This plugin allow to get word under cursor
 * Require: jQuery 1.7+
 * https://github.com/megahertz/jquery.get-word-by-event
 *
 * @licence GetWordByEvent jQuery plugin
 * Copyright (c) 2014 Alexey Prokhorov (https://github.com/megahertz)
 * Licensed under the MIT (LICENSE)
 *
 * @example
 * $('p').getWordByEvent('click', function(e, word) {
 *     alert('You have clicked "' + word + '" word')
 * });
 */
(function ($) {
	'use strict';

	/**
	 * @param {Event} event
	 * @returns {String}
	 */
	function getWordFromEvent(event) {
		var range, word;
		// IE
		if (document.body && document.body.createTextRange) {
			range = document.body.createTextRange();
			range.moveToPoint(event.clientX, event.clientY);
			range.expand('word');
			return range.text;
		// Firefox
		} else if (event.rangeParent && document.createRange) {
			range = document.createRange();
			range.setStart(event.rangeParent, event.rangeOffset);
			range.setEnd(event.rangeParent, event.rangeOffset);
			expandRangeToWord(range);
			word = range.toString();
			return word;
		// Webkit
		} else if (document.caretRangeFromPoint) {
			range = document.caretRangeFromPoint(event.clientX, event.clientY);
			expandRangeToWord(range);
			word = range.toString();
			return word;
		// Firefox for events without rangeParent
		} else if (document.caretPositionFromPoint) {
			var caret = document.caretPositionFromPoint(event.clientX, event.clientY);
			range = document.createRange();
			range.setStart(caret.offsetNode, caret.offset);
			range.setEnd(caret.offsetNode, caret.offset);
			expandRangeToWord(range);
			word = range.toString();
			range.detach();
			return word;
		} else {
			return null;
		}
	}

	/**
	 *
	 * @param {Range} range
	 * @returns {String}
	 */
	function expandRangeToWord(range) {
		while (range.startOffset > 0) {
			if (range.toString().indexOf(' ') === 0) {
				range.setStart(range.startContainer, range.startOffset + 1);
				break;
			}
			range.setStart(range.startContainer, range.startOffset - 1);
		}
		while (range.endOffset < range.endContainer.length && range.toString().indexOf(' ') == -1) {
			range.setEnd(range.endContainer, range.endOffset + 1);
		}
		return range.toString().trim();
	}

	/**
	 * Attach/detach event(s) to element and call handler when event is triggered
	 * @param {String} event
	 * @param {Function|Boolean} handler function(Event event, String word)
	 */
	$.fn.getWordByEvent = function(event, handler) {
		// Save last coordinates for events which does not have coordinates info (such as taphold)
		var coordinates = {};

		function handleCoordinates(e) {
			coordinates = { x: e.clientX, y: e.clientY };
		}

		function handle(e) {
			e = e.originalEvent || e;
			if (!e.clientX) {
				e.clientX = coordinates.x;
				e.clientY = coordinates.y;
			}
			handler.call(this, e, getWordFromEvent(e));
		}

		if (handler) {
			this.data('getWordByEvent.handleCoordinates', handleCoordinates);
			this.data('getWordByEvent.handler', handle);
			this.on('mousedown', handleCoordinates);
			this.on(event, handle);
		} else {
			this.off('mousedown', this.data('getWordByEvent.handleCoordinates'));
			this.off(event, this.data('getWordByEvent.handler'));
		}
	};
})(jQuery);