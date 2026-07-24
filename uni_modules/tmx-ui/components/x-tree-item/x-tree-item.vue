<script lang="ts" setup>
	import { ref, computed, watch, onMounted } from "vue"
	import { xConfig } from "../../config/xConfig.uts"
	import { flatChildrensId, setChildrenByid, findParentNode } from "../x-tree/util.uts"
	type callbackType = (id : string) => Promise<UTSJSONObject[]>
	
	/**
	 * @name 树子节点 xTreeItem
	 * @description xTree内部子组件，不可引用。
	 * @page /pages/index/tree
	 * @category 表单组件
	 * @constant 平台兼容
	 *	| Harmony | H5 | andriod | IOS | 小程序 | UTS | UNIAPP-X SDK | version |
		| --- | --- | --- | --- | --- | --- | --- | --- |
		| ☑ | ☑ | ☑️ | ☑️ | ☑️ | ☑️ | 4.76+ | 1.1.18 |
	 */
	defineOptions({ name: "xTreeItem" })

	type xTreeItemPropsType = {
		idList : string[],
		list : UTSJSONObject[],
		idKey : string,
		labelKey : string,
		folderId : string[],
		/**
		 * 选中时的高亮颜色或者主题
		 */
		color : string,
		/**
		 * 用户交互时是否禁用选中交互
		 * 但不影响你赋值选中
		 */
		disabledParentBox : boolean,
		/**
		 * 是否允许选中父级的时候把它的所有子级也选上。
		 */
		parentSelectedFullChildren : boolean,
		/**
		 * 是否停用选中功能，只能是已读状态了
		 * 但不影响你赋值选中
		 */
		disabled : boolean,
		beforeOpenFloder : callbackType,
		/**
		 * 关闭和打开时图标/图片地址
		 * 必须为长度2,第一项关闭时图标,第2项关闭时图标
		 */
		floaderIcon : string[],
		/**
		 * 是否显示默认前置开和关的图标
		 */
		showFloaderIcon : boolean,
		/**
		 * 是否显示选中的box图标
		 */
		showChecked : boolean,
		/**
		 * 选中与未选中时的图标/图片地址
		 * 必须为长度2,第一项未选中,第二项选中时图标
		 */
		checkedIcon : string[],
	}

	const emits = defineEmits<{
		/**
		 * 选中切换时触发
		 * @@param {string[]} ids - 打开的id数组 
		 */
		change : [ids : string[]],
		/**
		 * 节点标签被点击
		 * @@param {UTSJSONObject} item - 打开的id数组 
		 */
		childrenClick : [item : UTSJSONObject],
		/**
		 * 父节点展开和关闭
		 * @@param {string[]} ids - 打开的id数组 
		 */
		openFolderChange : [ids : string[], type : string, id : string],
		setChildrenData : [id : string, list : UTSJSONObject[]],
		'update:modelValue' : [ids : string[]],
	}>()

	const props = withDefaults(defineProps<xTreeItemPropsType>(), {
		idList: () : string[] => [] as string[],
		list: () : UTSJSONObject[] => [] as UTSJSONObject[],
		idKey: "id",
		labelKey: "text",
		folderId: () : string[] => [] as string[],
		color: "",
		disabledParentBox: false,
		parentSelectedFullChildren: false,
		disabled: false,
		beforeOpenFloder: (itemId : string) : Promise<UTSJSONObject[]> => {
			return Promise.resolve([] as UTSJSONObject[])
		},
		floaderIcon: () : string[] => ['folder-add-line','folder-open-fill'] as string[],
		showFloaderIcon: true,
		showChecked: false,
		checkedIcon: () : string[] => ['checkbox-blank-line','checkbox-fill'] as string[],
	})

	const nowIds = ref<string[]>([] as string[])
	const showChildren = ref<boolean>(true)
	const openFLoderIds = ref<string[]>([] as string[])
	const _list = ref<UTSJSONObject[]>([] as UTSJSONObject[])
	const nowediteItem = ref<UTSJSONObject | null>(null)
	const showModal = ref<boolean>(false)
	const nowediteItemText = ref<string>("")
	const nowediteItemId = ref<string>("")
	const editeType = ref<string>("change")//change/add
	const newItem = ref<UTSJSONObject | null>(null)

	const _fontSize = computed(() : string => {
		if (xConfig.fontScale == 1) return '16';
		return (16 * xConfig.fontScale).toString()
	})
	const _itemColor = computed(() : string => {
		if (xConfig.dark == 'dark') return 'white'
		return '#666';
	})

	const getChildren = (item : UTSJSONObject) : UTSJSONObject[] => {
		let c = item.getArray<UTSJSONObject>('children')
		if (c == null) return [] as UTSJSONObject[]
		return c;
	}
	const isInOpenFloder = (item : UTSJSONObject) : boolean => {
		let id = item.getString(props.idKey)
		if (id == null) {
			return false;
		}
		if (openFLoderIds.value.includes(id)) return true;
		return false
	}
	const isSelected = (item : UTSJSONObject) : boolean => {
		let id = item.getString(props.idKey)
		id = id == null ? "" : id;
		return nowIds.value.includes(id)
	}
	// 检查节点是否处于半选状态（部分子节点被选中）
	const isIndeterminate = (item : UTSJSONObject) : boolean => {
		// 如果节点已被选中，则不是半选状态
		if (isSelected(item)) return false;
		
		// 获取所有子节点ID
		let children = getChildren(item);
		if (children.length === 0) return false;
		
		// 获取所有可选的子节点ID
		let allChildrenIds = flatChildrensId(item, props.idKey);
		if (allChildrenIds.length === 0) return false;
		
		// 检查是否有部分子节点被选中
		let selectedChildrenCount = 0;
		for (let i = 0; i < allChildrenIds.length; i++) {
			if (nowIds.value.includes(allChildrenIds[i])) {
				selectedChildrenCount++;
				
			}
		}
		
		// 如果有部分子节点被选中（不是全部也不是零个），则是半选状态
		return selectedChildrenCount > 0 && selectedChildrenCount < allChildrenIds.length;
	}
	// 是否被禁用选择
	const isDisabled = (item : UTSJSONObject) : boolean => {
		let children = getChildren(item)
		// 自己的单独的禁用状态
		let disabled = item.getBoolean("disabled")
		disabled = disabled == null ? false : disabled
		// 父级
		if (children.length > 0) {
			return props.disabledParentBox || disabled || props.disabled
		}
		return disabled || props.disabled
	}

	const checkBoxClickEvent = (item : UTSJSONObject) => {

		if (isDisabled(item)) {
			return;
		}

		let id = item.getString(props.idKey)
		if (id == null) {
			return;
		}
		let idsIndex = -1;
		for (let i = 0; i < nowIds.value.length; i++) {
			if (nowIds.value[i] == id) {
				idsIndex = i;
				break;
			}
		}
		let sids = nowIds.value.slice(0)
		
		if(idsIndex>-1){
			sids.splice(idsIndex, 1)
			if (props.parentSelectedFullChildren && getChildren(item).length > 0) {
				let tmepids = flatChildrensId(item, props.idKey)
				sids = sids.filter((el) : boolean => !tmepids.includes(el))
			}
			
		//如果不存在选中中,它可能是半选状态或者完全没有选中.
		}else{
			sids.push(id)
			// 如果允许单选父,不影响子,需要单独处理,其它交给父组件处理
			if (props.parentSelectedFullChildren && getChildren(item).length > 0) {
				let tmepids = flatChildrensId(item, props.idKey)
				// tmepids = tmepids.filter((yid:string):string => !nowIds.value.includes(yid))
				sids = sids.concat(tmepids)
			}
		}
		
		
		
		
		

		nowIds.value = sids;
		// 更新父节点状态
		// updateParentNodeStatus();
		emits('change', nowIds.value.slice(0))
	}

	const childrenEvent = async (item : UTSJSONObject) : Promise<any> => {
		let id = item.getString(props.idKey)
		if (id == null) {
			return Promise.resolve(true);
		}

		emits("childrenClick", item)

		let idsIndex = -1;
		for (let i = 0; i < openFLoderIds.value.length; i++) {
			if (openFLoderIds.value[i] == id) {
				idsIndex = i;
				break;
			}
		}

		// 先判断 是不是父级，就是有没有children要展开。有就展开或者 关闭。没有不操作。
		let children = getChildren(item)
		let isOpenChildren = item.getArray('children')
		// 只有有children字段的才可以异步打开继续加载数据。
		if (isOpenChildren == null) {
			checkBoxClickEvent(item)
			return Promise.resolve(true);
		}

		if (idsIndex == -1 && children.length == 0) {

			// 没有子级再判断有没有异步加载的数据。
			let asyncList = await props.beforeOpenFloder(id)
			let resultList = asyncList as UTSJSONObject[]
			if (resultList.length > 0) {
				let n = setChildrenByid(_list.value, id, props.idKey, resultList)
				item.set('children', resultList)
				children = resultList

				emits('setChildrenData', id, _list.value)
			}
		}

		if (children.length == 0) {
			// 没有子级触发title事件就行或者直接选中操作。
			// checkBoxClickEvent(item)
			
			return Promise.resolve(true);
		}


		if (idsIndex > -1) {
			openFLoderIds.value.splice(idsIndex, 1)
		} else {
			openFLoderIds.value.push(id)
		}

		emits('openFolderChange', openFLoderIds.value.slice(0), idsIndex == -1 ? 'open' : 'close', id)

		return Promise.resolve(true);
	}
	const setChildrenDataEnd = (id : string, item : UTSJSONObject[]) => {
		let parent = findParentNode(_list.value, id, props.idKey) as UTSJSONObject | null

		if (parent == null) {
			let index = _list.value.findIndex((item) : boolean => id == item.getString(props.idKey)!)
			if (index > -1) {
				_list.value[index].set('children', item)
				emits('setChildrenData', id, _list.value)
			}
		} else {
			let index = _list.value.findIndex((item) : boolean => item!.getString(props.idKey) != parent!.getString(props.idKey)!)
			if (index > -1) {
				_list.value[index].set('children', item)
				emits('setChildrenData', id, _list.value)
			}
		}


	}

	const openFolderChangeEnd = (ids : string[], type : string, id : string) => {
		// let fullIds = new Set(openFLoderIds.value.concat(ids))
		// let idsreal = [] as string[]
		// fullIds.forEach((val : string) => {
		// 	idsreal.push(val)
		// })
		// openFLoderIds.value = ids
		emits('openFolderChange', ids.slice(0), type, id)
	}
	const childrenClickEnd = (item : UTSJSONObject) => {
		emits("childrenClick", item)
	}
	const changeEnd = (ids : string[]) => {
		// 当子节点状态变化时，更新当前节点状态
		nowIds.value = ids.slice(0);
		emits('change', ids.slice(0))
	}
	
	
	const isShowEdite = (item:UTSJSONObject):Boolean => {
		let showEditeLabel = item.getBoolean('showEdite')
		return showEditeLabel == true;
	}
	const isShowAdd = (item:UTSJSONObject):Boolean => {
		let showEditeLabel = item.getBoolean('showAdd')
		return showEditeLabel == true;
	}
	
	const isShowRemove = (item:UTSJSONObject):Boolean => {
		let showEditeLabel = item.getBoolean('showRemove')
		return showEditeLabel == true;
	}
	// 参数重命名为editeTypeVal，因为setup模式下无法通过this区分同名的ref和参数
	const showModalEdte = (item:UTSJSONObject,editeTypeVal:string) => {
		if(editeTypeVal == 'add'){
			let newitem = {
				disabled:false,
				showEdite:false,
				showRemove:true
			} as UTSJSONObject
			newitem.set(props.labelKey,"")
			newitem.set(props.idKey,"")
			newItem.value = newitem
		}
		let disabled = isDisabled(item)
		editeType.value = editeTypeVal;
		if(disabled) return;
		nowediteItem.value = item;
		if(editeTypeVal == 'change'){
			nowediteItemText.value = item.getString(props.labelKey)==null?'':item.getString(props.labelKey)!
			nowediteItemId.value = item.getString(props.idKey)==null?'':item.getString(props.idKey)!
		}else if(editeTypeVal == 'add'){
			nowediteItemId.value = ''
			nowediteItemText.value = ''
		}
		showModal.value = true;
	}
	const editeclick = () => {
		if(nowediteItem.value==null) return;
		if(nowediteItemText.value==''){
			uni.showToast({title:"标题不能为空",icon:'none'})
			return;
		}
		let item = nowediteItem.value!
		let id = item.getString(props.idKey)!
		
		if(editeType.value=='change'){
			item.set(props.labelKey,nowediteItemText.value)
			item.set(props.idKey,nowediteItemId.value)
			let index = _list.value.findIndex((item) : boolean => id == item.getString(props.idKey)!)
			if (index > -1) {
				_list.value[index] = item
				emits('setChildrenData', id, _list.value)
			}
		}else if(editeType.value == 'add'&&newItem.value!=null){
			id = nowediteItemId.value
			if(id==''){
				id = Math.random().toString(16).substring(1,20)
			}
			newItem.value!.set(props.labelKey,nowediteItemText.value)
			newItem.value!.set(props.idKey,id)
			
			let nextChildre = getChildren(item)
			if(nextChildre.length==0){
				newItem.value!.set('children',[newItem.value!] as UTSJSONObject[])
			}else{
				nextChildre.push(newItem.value! as UTSJSONObject)
			}
			let index = _list.value.findIndex((item) : boolean => id == item.getString(props.idKey)!)
			if (index > -1) {
				_list.value[index] = item
				emits('setChildrenData', id, _list.value)
			}
			
			nowediteItemId.value = ''
			nowediteItemText.value = ''
		}
		
		
	}
	const removeItem = (item:UTSJSONObject) => {
		console.log(_list.value)
		uni.showModal({
			title:"提醒",
			content:"确认删除?",
			success(res){
				if(res.confirm){
					
					let id = item.getString(props.idKey)!
					let index = _list.value.findIndex((item:UTSJSONObject) : boolean => id == item.getString(props.idKey)!)
					if (index > -1) {
						_list.value.splice(index,1)
						emits('setChildrenData', id, _list.value)
					}
				}
			}
		})
		
	}

	watch(() : string[] => props.folderId, (newVal : string[]) => {

		if (newVal.join(",") == openFLoderIds.value.join(",")) return;

		openFLoderIds.value = newVal
	})
	watch(() : UTSJSONObject[] => props.list, () => {
		// forceUpdate()
		_list.value = props.list;
	})
	watch(() : string[] => props.idList, (newVal : string[]) => {
		if (newVal.join(",") == nowIds.value.join(",")) return;
		nowIds.value = newVal;
	})

	onMounted(() => {
		_list.value = props.list;
		openFLoderIds.value = props.folderId
		nowIds.value = props.idList;
	})
</script>
<template>
	<view class="xTree">
		<!-- 'add'?'添加下级':'修改内容'"  -->
		<x-modal height="auto" @confirm="editeclick" 
		:title="editeType=='add'?(i18n!.t('tmui4x.tree.addTitle')):(i18n!.t('tmui4x.tree.changeTitle'))" 
		v-model:show="showModal">
			<view v-if="nowediteItem!=null">
				<x-input dark-bg-color="" height="48" v-model="nowediteItemText">
					<template v-slot:inputLeft>
						<!-- 标题值 -->
						<x-text _style="padding:0 12px">
						{{i18n!.t('tmui4x.tree.inputTitle')}}
						</x-text>
					</template>
				</x-input>
				<view style="height:10px"></view>
				<x-input dark-bg-color="" height="48" :placeholder="i18n!.t('tmui4x.tree.inputTips1')" v-model="nowediteItemId">
					<template v-slot:inputLeft>
						<!-- 标识ID -->
						<x-text _style="padding:0 12px">{{i18n!.t('tmui4x.tree.inputId')}}</x-text>
					</template>
				</x-input>
			</view>
		</x-modal>
		<view class="xTreeItemCell" v-for="(item,index) in _list" :key="index">
			<view 
			:hover-start-time="5" :hover-stay-time="30" :hover-class="getChildren(item).length>0?'hoverClass':'hoverClassOff'"
			@click.stop="childrenEvent(item)" :style="{height:'50px'}" class="xTreeItemCellWrap">
				<view class="xTreeItemCellWrapLeft">
					<view v-if="item.getArray('children')!=null&&showFloaderIcon" class="xTreeItemCellWrapLeftIcon">
						<x-icon color="#333333" v-if="!isInOpenFloder(item)" font-size="21"
							:name="floaderIcon[0]"></x-icon>
						<x-icon :color="color" v-if="isInOpenFloder(item)" font-size="21"
							:name="floaderIcon[1]"></x-icon>

					</view>
					<text :style="{color:isSelected(item)?color:_itemColor,fontSize:_fontSize}"
						class="xTreeItemCellWrapLeftText">{{item.getString(labelKey)}}</text>
				</view>
				<view v-if="isShowAdd(item)" :hover-start-time="5" :hover-stay-time="30" hover-class="hoverClass"
					@click.stop="showModalEdte(item,'add')" style="margin-right:10px;" >
					<x-icon :color="!isDisabled(item)?color:'#cecece'" font-size="21"
						name="add-large-line"></x-icon>
				</view>
				<view v-if="isShowEdite(item)" :hover-start-time="5" :hover-stay-time="30" hover-class="hoverClass"
					@click.stop="showModalEdte(item,'change')" style="margin-right:10px;" >
					<x-icon :color="!isDisabled(item)?color:'#cecece'" font-size="21"
						name="edit-box-line"></x-icon>
				</view>
				<view v-if="isShowRemove(item)" :hover-start-time="5" :hover-stay-time="30" hover-class="hoverClass"
					@click.stop="removeItem(item)" style="margin-right:10px;" >
					<x-icon :color="!isDisabled(item)?color:'#cecece'" font-size="21"
						name="delete-bin-2-line"></x-icon>
				</view>
				<view v-if="showChecked" :hover-start-time="5" :hover-stay-time="30" hover-class="hoverClass"
					@click.stop="checkBoxClickEvent(item)" :style="{opacity:isDisabled(item)?'0.3':'1'}">
					<x-icon v-if="parentSelectedFullChildren" :color="isSelected(item) || isIndeterminate(item) ? color : '#cecece'" font-size="21"
						:name="isSelected(item) ? checkedIcon[1] : (isIndeterminate(item) ? 'indeterminate-circle-line' : checkedIcon[0])"></x-icon>
					<x-icon v-else :color="isSelected(item) ? color : '#cecece'" font-size="21"
						:name="isSelected(item) ? checkedIcon[1] : checkedIcon[0]"></x-icon>
						
						
				</view>
			</view>
			<x-tree-item 
			:checkedIcon="checkedIcon"
			:showChecked="showChecked"
			:showFloaderIcon="showFloaderIcon"
			:floaderIcon="floaderIcon"
			@childrenClick="childrenClickEnd" @openFolderChange="openFolderChangeEnd"
				@setChildrenData="setChildrenDataEnd" :beforeOpenFloder="beforeOpenFloder"
				:parentSelectedFullChildren="parentSelectedFullChildren" :disabled="disabled"
				:disabledParentBox="disabledParentBox" @change="changeEnd" :idList="nowIds" :color="color"
				:folderId="openFLoderIds" :idKey="idKey" :labelKey="labelKey" style="margin-left: 30px;"
				v-if="isInOpenFloder(item)&&showChildren" :list="getChildren(item)" v-model="nowIds"></x-tree-item>
		</view>

	</view>
</template>
<style scoped>
	.hoverClass {
		background-color: rgba(0, 0, 0, 0.03);
	}
	.hoverClassOff {
		background-color: transparent;
	}
	.xTreeItemCellWrapLeftText {
		flex: 1;
		text-overflow: ellipsis;
		lines: 2;
		/* #ifndef APP */
		display: -webkit-box;
		-webkit-box-orient: vertical;
		overflow: hidden;
		word-break: break-all;
		-webkit-line-clamp: 2;
		/* #endif */
	}

	.xTreeItemCellWrapLeftIcon {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		align-items: center;
		width: 30px;
	}

	.xTreeItemCellWrapLeft {
		flex: 1;
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		align-items: center;
	}

	.xTreeItemCellWrap {
		display: flex;
		flex-direction: row;
		justify-content: space-between;
		align-items: center;
		border-bottom: 1px solid rgba(0, 0, 0, 0.03);

	}
</style>
