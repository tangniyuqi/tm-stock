function isFuncString(s) {
	var t = s.trim();
	if (t.indexOf('function') === 0) return true;
	var arrowIdx = t.indexOf('=>');
	if (arrowIdx > 0) {
		var after = t.substring(arrowIdx + 2).trim();
		if (after.charAt(0) === '{' || after.charAt(0) === '(' ||
			after.charAt(0) === '[' || after.indexOf('.') > -1 ||
			after.indexOf('+') > -1 || after.indexOf('return') > -1) {
			return true;
		}
	}
	return false;
}

function reviveFunctions(obj) {
	if (typeof obj !== 'object' || obj === null) return;
	var keys = Object.keys(obj);
	for (var i = 0; i < keys.length; i++) {
		var key = keys[i];
		var val = obj[key];
		if (typeof val === 'string') {
			if (isFuncString(val)) {
				try {
					obj[key] = (new Function('return (' + val + ')'))();
				} catch (e) {}
			}
		} else if (Array.isArray(val)) {
			for (var j = 0; j < val.length; j++) {
				if (typeof val[j] === 'string') {
					if (isFuncString(val[j])) {
						try {
							val[j] = (new Function('return (' + val[j] + ')'))();
						} catch (e) {}
					}
				} else if (typeof val[j] === 'object' && val[j] !== null) {
					reviveFunctions(val[j]);
				}
			}
		} else if (typeof val === 'object') {
			reviveFunctions(val);
		}
	}
}

function unCoderOptsStrToJson(str) {
	if (typeof str !== 'string' || str === '') return null;
	var result = null;
	try {
		result = (new Function('return (' + str + ')'))();
	} catch (e) {
		try {
			result = JSON.parse(str);
		} catch (e2) {
			return null;
		}
	}
	if (result !== null && typeof result === 'object') {
		reviveFunctions(result);
	}
	return result;
}
