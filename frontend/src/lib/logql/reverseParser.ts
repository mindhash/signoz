/* eslint-disable  */
// @ts-ignore
// @ts-nocheck
import {
	QueryOperatorsMultiVal,
	QueryOperatorsSingleVal,
} from 'lib/logql/tokens';
export const reverseParser = (
	parserQueryArr: { type: string; value: any }[] = [],
) => {
	console.log('reverseParser ')
	let queryString = '';
	// let lastKeyType = '';
	// let lastKeyVal = '';
	// parserQueryArr.forEach((query) => {
		
	// 	if (queryString) {
	// 		queryString += ' ';
	// 	}
	// 	console.log('lastKeyType:', lastKeyType)
	// 	console.log('current type:', key.type)
	// 	// if (lastKeyType === 'QUERY_OPERATOR' && key.type === 'CONDITIONAL_OPERATOR') {
	// 	// 	if (Object.values(QueryOperatorsMultiVal).includes(lastKeyVal)) {
	// 	// 		queryString += "()";
	// 	// 	} else if (Object.values(QueryOperatorsSingleVal).includes(lastKeyVal)){
	// 	// 		queryString += 'null';
	// 	// 	}
	// 	// }

	// 	if (Array.isArray(query.value) ) {
	// 		if (query.value.length > 0) {
	// 			queryString += `(${query.value.map((val) => `'${val}'`).join(',')})`;
	// 		} else {
	// 			if (query.type === 'QUERY_VALUE') queryString += '()'
	// 		}
			
	// 	} else {
	// 		if (query.type === 'QUERY_VALUE' && (!query.value || query.value === null)) {
	// 			queryString += 'null';
	// 		} else {
	// 			queryString += query.value;
	// 		}
			
	// 	}
	// 	lastKeyType = query.type;
	// 	lastKeyVal = query.value;
	// });

	// if (lastKeyType === 'QUERY_OPERATOR') {
	// 	if (Object.values(QueryOperatorsMultiVal).includes(lastKeyVal)) {
	// 				queryString += "()";
	// 			} else if (Object.values(QueryOperatorsSingleVal).includes(lastKeyVal)){
	// 				queryString += 'null';
	// 			}
	// }

	let source: string = '';
	let op: string = '';
	let val: string = '';
	let cond: string = ''; 
	let expr: string = '';

	//console.log(' parserQueryArr:', parserQueryArr);
	parserQueryArr.forEach((query) => {
		switch(query.type) {
			case "QUERY_KEY": 
				if (source !== '' && op !== '' && val != '' && cond != '') {
					console.log(' expr:', source + ' ' + op + ' ' + val + ' ' + cond)
					queryString += ' ' +  source + ' ' + op + ' ' + val + ' ' + cond ;
					source = '';
					op = '';
					val = '';
					cond =''; 

				} 	
				source = query.value;
				
				break;
			case "QUERY_OPERATOR":
				op = query.value;
				break;
			case "QUERY_VALUE":
				if (Array.isArray(query.value) ) {
					if (query.value.length > 0) {
						val += `(${query.value.map((val) => `'${val}'`).join(',')})`;
					} else {
						if (query.type === 'QUERY_VALUE') {
							val += '()'
							
					
						};
					}
					
				} else {
					val = query.value || '0';
				}
				break;
			case "CONDITIONAL_OPERATOR":
				cond = query.value;
				break;
		}		
		
	});

	if (source !== '' ) {
		op = !op ? 'IN': op;
		const defaultVal = Object.values(QueryOperatorsMultiVal).includes(op)? '()': 0;
		val = !val? defaultVal: val;
		queryString  += ' ' + source + ' ' + op + ' ' +  val;
	}
	
	// console.log(queryString);
	return queryString;
};

export default reverseParser;
