package parser

import (
	"sort"
	"strings"

	"github.com/therecipe/qt/internal/utils"
)

type Function struct {
	Name            string       `xml:"name,attr"`
	Fullname        string       `xml:"fullname,attr"`
	Href            string       `xml:"href,attr"`
	Status          string       `xml:"status,attr"`
	Access          string       `xml:"access,attr"`
	Filepath        string       `xml:"filepath,attr"`
	Virtual         string       `xml:"virtual,attr"`
	Meta            string       `xml:"meta,attr"`
	Static          bool         `xml:"static,attr"`
	Overload        bool         `xml:"overload,attr"`
	OverloadNumber  string       `xml:"overload-number,attr"`
	Output          string       `xml:"type,attr"`
	Signature       string       `xml:"signature,attr"`
	Parameters      []*Parameter `xml:"parameter"`
	Brief           string       `xml:"brief,attr"`
	Since           string       `xml:"since,attr"`
	SignalMode      string
	TemplateModeJNI string
	Default         bool
	TmpName         string
	Export          bool
	NeedsFinalizer  bool
	Container       string
	TemplateModeGo  string
	NonMember       bool
	NoMocDeduce     bool
	AsError         bool
	Synthetic       bool
	Checked         bool
	Exception       bool
	IsMap           bool
	OgParameters    []Parameter
	IsMocFunction   bool
}

type Parameter struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"left,attr"`
}

func (f *Function) Class() (*Class, bool) {
	var class, exist = State.ClassMap[f.ClassName()]
	return class, exist
}

func (f *Function) ClassName() string {
	var s = strings.Split(f.Fullname, "::")
	if len(s) == 3 {
		return s[1]
	}
	return s[0]
}

//TODO: multipoly [][]string
//TODO: connect/disconnect slot functions + add necessary SIGNAL_* functions (check first if really needed)
//TODO: replace self poly deduction with overridden methods ?

func (f *Function) PossiblePolymorphic(self bool) ([]string, string) {
	var out = make([]string, 0)

	var params = func() []*Parameter {
		if self {
			return []*Parameter{{Name: "ptr", Value: f.ClassName()}}
		}
		return f.Parameters
	}()

	var fc, _ = f.Class()

	for _, p := range params {
		var c, exist = State.ClassMap[CleanValue(p.Value)]
		if !exist {
			continue
		}

		for _, class := range SortedClassesForModule(fc.Module, false) {
			if class.IsPolymorphic() && class.IsSubClassOf(c.Name) {
				out = append(out, class.Name)
			}
		}

		//TODO: multipoly
		if len(out) > 0 {
			sort.Stable(sort.StringSlice(out))
			out = append(out, c.Name)
			return out, CleanName(p.Name, p.Value)
		}
	}

	return out, ""
}

func (f *Function) IsJNIGeneric() bool {

	if f.ClassName() == "QAndroidJniObject" {
		switch f.Name {
		case
			"callMethod",
			"callStaticMethod",

			"getField",
			//"setField", -> uses interface{} if not generic

			"getStaticField",
			//"setStaticField", -> uses interface{} if not generic

			"getObjectField",

			"getStaticObjectField",

			"callObjectMethod",
			"callStaticObjectMethod":
			{
				return true
			}

		case "setStaticField":
			{
				if f.OverloadNumber == "2" || f.OverloadNumber == "4" {
					return true
				}
			}
		}
	}

	return false
}

func (f *Function) IsSupported() bool {

	if utils.QT_VERSION() == "5.8.0" {
		if f.Fullname == "QJSEngine::newQMetaObject" && f.OverloadNumber == "2" ||
			f.Fullname == "QScxmlTableData::instructions" || f.Fullname == "QScxmlTableData::dataNames" ||
			f.Fullname == "QScxmlTableData::stateMachineTable" ||
			f.Fullname == "QTextToSpeech::voiceChanged" {
			if !strings.Contains(f.Access, "unsupported") {
				f.Access = "unsupported_isBlockedFunction"
			}
			return false
		}
	}

	switch {
	case
		(f.ClassName() == "QAccessibleObject" || f.ClassName() == "QAccessibleInterface" || f.ClassName() == "QAccessibleWidget" || //QAccessible::State -> quint64
			f.ClassName() == "QAccessibleStateChangeEvent") && (f.Name == "state" || f.Name == "changedStates" || f.Name == "m_changedStates" || f.Name == "setM_changedStates" || f.Meta == CONSTRUCTOR),

		f.Fullname == "QPixmapCache::find" && f.OverloadNumber == "4", //Qt::Key -> int
		(f.Fullname == "QPixmapCache::remove" || f.Fullname == "QPixmapCache::insert") && f.OverloadNumber == "2",
		f.Fullname == "QPixmapCache::replace",

		f.Fullname == "QNdefFilter::appendRecord" && !f.Overload, //QNdefRecord::TypeNameFormat -> uint

		f.ClassName() == "QSimpleXmlNodeModel" && f.Meta == CONSTRUCTOR,

		f.Fullname == "QSGMaterialShader::attributeNames",

		f.ClassName() == "QVariant" && (f.Name == "value" || f.Name == "canConvert"), //needs template

		f.Fullname == "QNdefRecord::isRecordType", f.Fullname == "QScriptEngine::scriptValueFromQMetaObject", //needs template
		f.Fullname == "QScriptEngine::fromScriptValue", f.Fullname == "QJSEngine::fromScriptValue",

		f.ClassName() == "QMetaType" && //needs template
			(f.Name == "hasRegisteredComparators" || f.Name == "registerComparators" ||
				f.Name == "hasRegisteredConverterFunction" || f.Name == "registerConverter" ||
				f.Name == "registerEqualsComparator"),

		State.ClassMap[f.ClassName()].Module == MOC && f.Name == "metaObject", //needed for qtmoc

		f.Fullname == "QSignalBlocker::QSignalBlocker" && f.OverloadNumber == "3", //undefined symbol

		(State.ClassMap[f.ClassName()].IsSubClassOf("QCoreApplication") ||
			f.ClassName() == "QAudioInput" || f.ClassName() == "QAudioOutput") && f.Name == "notify", //redeclared (name collision with QObject)

		f.Fullname == "QGraphicsItem::isBlockedByModalPanel", //** problem

		f.Name == "surfaceHandle", //QQuickWindow && QQuickView //unsupported_cppType(QPlatformSurface)

		f.Name == "QDesignerFormWindowInterface" || f.Name == "QDesignerFormWindowManagerInterface" || f.Name == "QDesignerWidgetBoxInterface", //unimplemented virtual

		f.Fullname == "QNdefNfcSmartPosterRecord::titleRecords", //T<T> output with unsupported output for *_atList
		f.Fullname == "QHelpEngineCore::filterAttributeSets", f.Fullname == "QHelpSearchEngine::query", f.Fullname == "QHelpSearchQueryWidget::query",
		f.Fullname == "QPluginLoader::staticPlugins", f.Fullname == "QSslConfiguration::ellipticCurves", f.Fullname == "QSslConfiguration::supportedEllipticCurves",
		f.Fullname == "QTextFormat::lengthVectorProperty", f.Fullname == "QTextTableFormat::columnWidthConstraints", f.Fullname == "QHelpContentWidget::selectedIndexes",

		f.Fullname == "QListView::indexesMoved", f.Fullname == "QAudioInputSelectorControl::availableInputs", f.Fullname == "QScxmlStateMachine::initialValuesChanged",
		f.Fullname == "QAudioOutputSelectorControl::availableOutputs", f.Fullname == "QQuickWebEngineProfile::downloadFinished",
		f.Fullname == "QQuickWindow::closing", f.Fullname == "QQuickWebEngineProfile::downloadRequested", f.Fullname == "QWebEnginePage::fullScreenRequested",

		f.Fullname == "QApplication::autoMaximizeThreshold", f.Fullname == "QApplication::setAutoMaximizeThreshold",

		strings.Contains(f.Access, "unsupported"):
		{
			if !strings.Contains(f.Access, "unsupported") {
				f.Access = "unsupported_isBlockedFunction"
			}
			return false
		}
	}

	//generic blocked
	//TODO: also check _setList _atList _newList _keyList instead ?
	var genName = strings.TrimPrefix(f.Name, "__")
	if strings.HasPrefix(genName, "registeredTimers") || strings.HasPrefix(genName, "countriesForLanguage") ||
		strings.HasPrefix(genName, "writingSystem") || strings.HasPrefix(genName, "textList") ||
		strings.HasPrefix(genName, "attributes") || strings.HasPrefix(genName, "additionalFormats") ||
		strings.HasPrefix(genName, "rawHeaderPairs") || strings.HasPrefix(genName, "draw") || strings.HasPrefix(genName, "tabs") ||
		strings.HasPrefix(genName, "QInputMethodEvent_attributes") || strings.HasPrefix(genName, "selections") || strings.HasPrefix(genName, "setSelections") ||
		strings.HasPrefix(genName, "formats") || strings.HasPrefix(genName, "setAdditionalFormats") || strings.HasPrefix(genName, "setFormats") ||
		strings.HasPrefix(genName, "setTabs") || strings.HasPrefix(genName, "extraSelections") ||
		strings.HasPrefix(genName, "setExtraSelections") || strings.HasPrefix(genName, "setButtonLayout") ||
		strings.HasPrefix(genName, "setWhiteList") || strings.HasPrefix(genName, "whiteList") ||
		strings.HasPrefix(genName, "supportedViewfinderFrameRateRanges") || strings.HasPrefix(genName, "hits") ||
		strings.HasPrefix(genName, "featureTypes") || strings.HasPrefix(genName, "supportedPaperSources") ||
		strings.HasPrefix(genName, "setTextureData") || strings.HasPrefix(genName, "textureData") ||
		strings.HasPrefix(genName, "QCustom3DVolume_textureData") || strings.HasPrefix(genName, "createTextureData") ||
		strings.Contains(genName, "alternateSubjectNames") || strings.HasPrefix(genName, "fromVariantMap") ||
		strings.HasPrefix(genName, "QScxmlDataModel") {
		return false
	}

	if State.Minimal {
		return f.Export || f.Meta == DESTRUCTOR || f.Fullname == "QObject::destroyed" || strings.HasPrefix(f.Name, TILDE)
	}

	return true
}

func (f *Function) IsDerivedFromVirtual() bool {
	var class, ok = f.Class()
	if !ok {
		return false
	}

	if f.Virtual != "non" {
		return true
	}

	for _, bc := range class.GetAllBases() {
		if bclass, exists := State.ClassMap[bc]; exists {
			for _, bcf := range bclass.Functions {
				if f.Name == bcf.Name && bcf.Virtual != "non" {
					return true
				}
			}
		}
	}

	return false
}

func (f *Function) IsDerivedFromImpure() bool {
	var class, _ = f.Class()

	if f.Virtual != PURE {
		return true
	}

	for _, bc := range class.GetAllBases() {
		if bclass, exists := State.ClassMap[bc]; exists {
			for _, bcf := range bclass.Functions {
				if f.Name == bcf.Name {
					return bcf.Virtual != PURE
				}
			}
		}
	}

	return false
}
