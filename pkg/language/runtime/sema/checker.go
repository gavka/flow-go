package sema

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
)

const ArgumentLabelNotRequired = "_"
const InitializerIdentifier = "init"
const SelfIdentifier = "self"
const BeforeIdentifier = "before"
const ResultIdentifier = "result"

type functionContext struct {
	returnType Type
	loops      int
}

var beforeType = &FunctionType{
	ParameterTypes: []Type{&AnyType{}},
	ReturnType:     &AnyType{},
	Apply: func(types []Type) Type {
		return types[0]
	},
}

// Checker

type Checker struct {
	Program          *ast.Program
	errors           []error
	valueActivations *activations.Activations
	typeActivations  *activations.Activations
	functionContexts []*functionContext
	Globals          map[string]*Variable
	inCondition      bool
	Origins          *Origins
	// TODO: refactor into fields on AST?
	FunctionDeclarationFunctionTypes   map[*ast.FunctionDeclaration]*FunctionType
	VariableDeclarationValueTypes      map[*ast.VariableDeclaration]Type
	AssignmentStatementTargetTypes     map[*ast.AssignmentStatement]Type
	StructureDeclarationTypes          map[*ast.StructureDeclaration]*StructureType
	InitializerFunctionTypes           map[*ast.InitializerDeclaration]*FunctionType
	FunctionExpressionFunctionType     map[*ast.FunctionExpression]*FunctionType
	InvocationExpressionParameterTypes map[*ast.InvocationExpression][]Type
	InterfaceDeclarationTypes          map[*ast.InterfaceDeclaration]*InterfaceType
}

func NewChecker(program *ast.Program) *Checker {
	typeActivations := &activations.Activations{}
	typeActivations.Push(baseTypes)

	valueActivations := &activations.Activations{}
	valueActivations.Push(hamt.NewMap())

	return &Checker{
		Program:                            program,
		valueActivations:                   valueActivations,
		typeActivations:                    typeActivations,
		Globals:                            map[string]*Variable{},
		Origins:                            NewOrigins(),
		FunctionDeclarationFunctionTypes:   map[*ast.FunctionDeclaration]*FunctionType{},
		VariableDeclarationValueTypes:      map[*ast.VariableDeclaration]Type{},
		AssignmentStatementTargetTypes:     map[*ast.AssignmentStatement]Type{},
		StructureDeclarationTypes:          map[*ast.StructureDeclaration]*StructureType{},
		InitializerFunctionTypes:           map[*ast.InitializerDeclaration]*FunctionType{},
		FunctionExpressionFunctionType:     map[*ast.FunctionExpression]*FunctionType{},
		InvocationExpressionParameterTypes: map[*ast.InvocationExpression][]Type{},
		InterfaceDeclarationTypes:          map[*ast.InterfaceDeclaration]*InterfaceType{},
	}
}

type ValueDeclaration interface {
	DeclarationName() string
	DeclarationType() Type
	DeclarationKind() common.DeclarationKind
	DeclarationPosition() ast.Position
	DeclarationIsConstant() bool
	DeclarationArgumentLabels() []string
}

func (checker *Checker) DeclareValue(declaration ValueDeclaration) error {
	checker.errors = nil
	identifier := declaration.DeclarationName()
	variable := checker.declareVariable(
		identifier,
		declaration.DeclarationType(),
		declaration.DeclarationKind(),
		declaration.DeclarationPosition(),
		declaration.DeclarationIsConstant(),
		declaration.DeclarationArgumentLabels(),
	)
	checker.recordVariableOrigin(identifier, variable)
	return checker.checkerError()
}

func (checker *Checker) Check() error {
	checker.errors = nil
	checker.Program.Accept(checker)
	return checker.checkerError()
}

func (checker *Checker) checkerError() error {
	if len(checker.errors) > 0 {
		return &CheckerError{
			Errors: checker.errors,
		}
	}
	return nil
}

func (checker *Checker) report(errs ...error) {
	checker.errors = append(checker.errors, errs...)
}

func (checker *Checker) IsSubType(subType Type, superType Type) bool {
	if subType.Equal(superType) {
		return true
	}

	if _, ok := superType.(*AnyType); ok {
		return true
	}

	if _, ok := subType.(*NeverType); ok {
		return true
	}

	switch typedSuperType := superType.(type) {
	case *IntegerType:
		switch subType.(type) {
		case *IntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type,
			*UInt8Type, *UInt16Type, *UInt32Type, *UInt64Type:

			return true

		default:
			return false
		}

	case *OptionalType:
		optionalSubType, ok := subType.(*OptionalType)
		if !ok {
			// T <: U? if T <: U
			return checker.IsSubType(subType, typedSuperType.Type)
		}
		// optionals are covariant: T? <: U? if T <: U
		return checker.IsSubType(optionalSubType.Type, typedSuperType.Type)

	case *InterfaceType:
		structureSubType, ok := subType.(*StructureType)
		if !ok {
			return false
		}
		// TODO: optimize, use set
		for _, conformance := range structureSubType.Conformances {
			if typedSuperType.Equal(conformance) {
				return true
			}
		}
		return false
	}

	// TODO: functions

	return false
}

func (checker *Checker) IndexableElementType(ty Type) Type {
	switch ty := ty.(type) {
	case ArrayType:
		return ty.elementType()
	}

	return nil
}

func (checker *Checker) IsIndexingType(indexingType Type, indexedType Type) bool {
	switch indexedType.(type) {
	// arrays can be index with integers
	case ArrayType:
		return checker.IsSubType(indexingType, &IntegerType{})
	}

	return false
}

func (checker *Checker) setVariable(name string, variable *Variable) {
	checker.valueActivations.Set(name, variable)
}

func (checker *Checker) setType(name string, ty Type) {
	checker.typeActivations.Set(name, ty)
}

func (checker *Checker) findVariable(name string) *Variable {
	value := checker.valueActivations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (checker *Checker) FindType(name string) Type {
	value := checker.typeActivations.Find(name)
	if value == nil {
		return nil
	}
	ty, ok := value.(Type)
	if !ok {
		return nil
	}
	return ty
}

func (checker *Checker) pushActivations() {
	checker.valueActivations.PushCurrent()
	checker.typeActivations.PushCurrent()
}

func (checker *Checker) popActivations() {
	checker.valueActivations.Pop()
	checker.typeActivations.Pop()
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {

	// pre-declare interfaces, structures, and functions (check afterwards)

	for _, interfaceDeclaration := range program.InterfaceDeclarations() {
		checker.declareInterfaceDeclaration(interfaceDeclaration)
	}

	for _, structureDeclaration := range program.StructureDeclarations() {
		checker.declareStructureDeclaration(structureDeclaration)
	}

	for _, functionDeclaration := range program.FunctionDeclarations() {
		checker.declareFunctionDeclaration(functionDeclaration)
	}

	// check all declarations

	for _, declaration := range program.Declarations {
		declaration.Accept(checker)
		checker.declareGlobal(declaration)
	}

	return nil
}

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {

	checker.checkFunctionAccessModifier(declaration)

	// global functions were previously declared, see `declareFunctionDeclaration`

	functionType := checker.FunctionDeclarationFunctionTypes[declaration]
	if functionType == nil {
		functionType = checker.declareFunctionDeclaration(declaration)
	}

	checker.checkFunction(
		declaration.Parameters,
		functionType,
		declaration.FunctionBlock,
	)

	return nil
}

func (checker *Checker) declareFunctionDeclaration(declaration *ast.FunctionDeclaration) *FunctionType {

	functionType := checker.functionType(declaration.Parameters, declaration.ReturnType)
	argumentLabels := checker.argumentLabels(declaration.Parameters)

	checker.FunctionDeclarationFunctionTypes[declaration] = functionType

	checker.declareFunction(
		declaration.Identifier,
		declaration.IdentifierPos,
		functionType,
		argumentLabels,
		true,
	)

	return functionType
}

func (checker *Checker) checkFunctionAccessModifier(declaration *ast.FunctionDeclaration) {
	switch declaration.Access {
	case ast.AccessNotSpecified, ast.AccessPublic:
		return
	default:
		checker.report(
			&InvalidAccessModifierError{
				DeclarationKind: common.DeclarationKindFunction,
				Access:          declaration.Access,
				Pos:             declaration.StartPosition(),
			},
		)
	}
}

func (checker *Checker) argumentLabels(parameters []*ast.Parameter) []string {
	argumentLabels := make([]string, len(parameters))

	for i, parameter := range parameters {
		argumentLabel := parameter.Label
		// if no argument label is given, the parameter name
		// is used as the argument labels and is required
		if argumentLabel == "" {
			argumentLabel = parameter.Identifier
		}
		argumentLabels[i] = argumentLabel
	}

	return argumentLabels
}

func (checker *Checker) checkFunction(
	parameters []*ast.Parameter,
	functionType *FunctionType,
	functionBlock *ast.FunctionBlock,
) {
	checker.pushActivations()
	defer checker.popActivations()

	// check argument labels
	checker.checkArgumentLabels(parameters)

	checker.declareParameters(parameters, functionType.ParameterTypes)

	if functionBlock != nil {
		func() {
			// check the function's block
			checker.enterFunction(functionType)
			defer checker.leaveFunction()

			checker.visitFunctionBlock(functionBlock, functionType.ReturnType)
		}()
	}
}

// checkArgumentLabels checks that all argument labels (if any) are unique
//
func (checker *Checker) checkArgumentLabels(parameters []*ast.Parameter) {

	argumentLabelPositions := map[string]ast.Position{}

	for _, parameter := range parameters {
		label := parameter.Label
		if label == "" || label == ArgumentLabelNotRequired {
			continue
		}

		labelPos := *parameter.LabelPos

		if previousPos, ok := argumentLabelPositions[label]; ok {
			checker.report(
				&RedeclarationError{
					Kind:        common.DeclarationKindArgumentLabel,
					Name:        label,
					Pos:         labelPos,
					PreviousPos: &previousPos,
				},
			)
		}

		argumentLabelPositions[label] = labelPos
	}
}

// declareParameters declares a constant for each parameter,
// ensuring names are unique and constants don'T already exist
//
func (checker *Checker) declareParameters(parameters []*ast.Parameter, parameterTypes []Type) {

	depth := checker.valueActivations.Depth()

	for i, parameter := range parameters {
		identifier := parameter.Identifier

		// check if variable with this identifier is already declared in the current scope
		existingVariable := checker.findVariable(identifier)
		if existingVariable != nil && existingVariable.Depth == depth {
			checker.report(
				&RedeclarationError{
					Kind:        common.DeclarationKindParameter,
					Name:        identifier,
					Pos:         parameter.IdentifierPos,
					PreviousPos: existingVariable.Pos,
				},
			)

			continue
		}

		parameterType := parameterTypes[i]

		variable := &Variable{
			Kind:       common.DeclarationKindParameter,
			IsConstant: true,
			Type:       parameterType,
			Depth:      depth,
			Pos:        &parameter.IdentifierPos,
		}
		checker.setVariable(identifier, variable)
		checker.recordVariableOrigin(identifier, variable)
	}
}

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	checker.visitVariableDeclaration(declaration, false)
	return nil
}

func (checker *Checker) visitVariableDeclaration(declaration *ast.VariableDeclaration, isOptionalBinding bool) {
	valueType := declaration.Value.Accept(checker).(Type)

	// if the variable declaration is a optional binding, the value must be optional

	var valueIsOptional bool
	var optionalValueType *OptionalType

	if isOptionalBinding {
		optionalValueType, valueIsOptional = valueType.(*OptionalType)
		if !valueIsOptional {
			checker.report(
				&TypeMismatchError{
					ExpectedType: &OptionalType{},
					ActualType:   valueType,
					StartPos:     declaration.Value.StartPosition(),
					EndPos:       declaration.Value.EndPosition(),
				},
			)
		}
	}

	declarationType := valueType

	// does the declaration have an explicit type annotation?
	if declaration.Type != nil {
		declarationType = checker.ConvertType(declaration.Type)

		// check the value type is a subtype of the declaration type
		if declarationType != nil && valueType != nil && !valueType.Equal(&InvalidType{}) {

			if isOptionalBinding {
				if optionalValueType != nil &&
					(optionalValueType.Equal(declarationType) ||
						!checker.IsSubType(optionalValueType.Type, declarationType)) {

					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   optionalValueType.Type,
							StartPos:     declaration.Value.StartPosition(),
							EndPos:       declaration.Value.EndPosition(),
						},
					)
				}

			} else {
				if !checker.IsSubType(valueType, declarationType) {
					checker.report(
						&TypeMismatchError{
							ExpectedType: declarationType,
							ActualType:   valueType,
							StartPos:     declaration.Value.StartPosition(),
							EndPos:       declaration.Value.EndPosition(),
						},
					)
				}
			}
		}
	} else if isOptionalBinding && optionalValueType != nil {
		declarationType = optionalValueType.Type
	}

	checker.VariableDeclarationValueTypes[declaration] = declarationType

	variable := checker.declareVariable(
		declaration.Identifier,
		declarationType,
		declaration.DeclarationKind(),
		declaration.IdentifierPosition(),
		declaration.IsConstant,
		nil,
	)
	checker.recordVariableOrigin(declaration.Identifier, variable)
}

func (checker *Checker) declareVariable(
	identifier string,
	ty Type,
	kind common.DeclarationKind,
	pos ast.Position,
	isConstant bool,
	argumentLabels []string,
) *Variable {

	depth := checker.valueActivations.Depth()

	// check if variable with this name is already declared in the current scope
	existingVariable := checker.findVariable(identifier)
	if existingVariable != nil && existingVariable.Depth == depth {
		checker.report(
			&RedeclarationError{
				Kind:        kind,
				Name:        identifier,
				Pos:         pos,
				PreviousPos: existingVariable.Pos,
			},
		)
	}

	// variable with this name is not declared in current scope, declare it
	variable := &Variable{
		Kind:           kind,
		IsConstant:     isConstant,
		Depth:          depth,
		Type:           ty,
		Pos:            &pos,
		ArgumentLabels: argumentLabels,
	}
	checker.setVariable(identifier, variable)
	return variable
}

func (checker *Checker) declareGlobal(declaration ast.Declaration) {
	name := declaration.DeclarationName()
	checker.Globals[name] = checker.findVariable(name)
}

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {

	checker.pushActivations()
	defer checker.popActivations()

	checker.visitStatements(block.Statements)

	return nil
}

func (checker *Checker) visitStatements(statements []ast.Statement) {

	// check all statements
	for _, statement := range statements {

		// check statement is not a local structure or interface declaration

		if _, ok := statement.(*ast.StructureDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:     common.DeclarationKindStructure,
					StartPos: statement.StartPosition(),
					EndPos:   statement.EndPosition(),
				},
			)

			continue
		}

		if _, ok := statement.(*ast.InterfaceDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:     common.DeclarationKindInterface,
					StartPos: statement.StartPosition(),
					EndPos:   statement.EndPosition(),
				},
			)

			continue
		}

		// check statement

		statement.Accept(checker)
	}
}

func (checker *Checker) VisitFunctionBlock(functionBlock *ast.FunctionBlock) ast.Repr {
	// NOTE: see visitFunctionBlock
	panic(&errors.UnreachableError{})
}

func (checker *Checker) visitFunctionBlock(functionBlock *ast.FunctionBlock, returnType Type) {

	checker.pushActivations()
	defer checker.popActivations()

	checker.visitConditions(functionBlock.PreConditions)

	// NOTE: not checking block as it enters a new scope
	// and post-conditions need to be able to refer to block's declarations

	checker.visitStatements(functionBlock.Block.Statements)

	// if there is a post-condition, declare the function `before`

	// TODO: improve: only declare when a condition actually refers to `before`?

	if len(functionBlock.PostConditions) > 0 {
		checker.declareBefore()
	}

	// if there is a return type, declare the constant `result`
	// which has the return type

	if _, ok := returnType.(*VoidType); !ok {
		checker.declareVariable(
			ResultIdentifier,
			returnType,
			common.DeclarationKindConstant,
			ast.Position{},
			true,
			nil,
		)
		// TODO: record origin - but what position?
	}

	checker.visitConditions(functionBlock.PostConditions)
}

func (checker *Checker) declareBefore() {

	checker.declareVariable(
		BeforeIdentifier,
		beforeType,
		common.DeclarationKindFunction,
		ast.Position{},
		true,
		nil,
	)
	// TODO: record origin – but what position?
}

func (checker *Checker) visitConditions(conditions []*ast.Condition) {

	// flag the checker to be inside a condition.
	// this flag is used to detect illegal expressions,
	// see e.g. VisitFunctionExpression

	wasInCondition := checker.inCondition
	checker.inCondition = true
	defer func() {
		checker.inCondition = wasInCondition
	}()

	// check all conditions: check the expression
	// and ensure the result is boolean

	for _, condition := range conditions {
		condition.Accept(checker)
	}
}

func (checker *Checker) VisitCondition(condition *ast.Condition) ast.Repr {

	// check test expression is boolean

	testType := condition.Test.Accept(checker).(Type)

	if !testType.Equal(&InvalidType{}) && !checker.IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     condition.Test.StartPosition(),
				EndPos:       condition.Test.EndPosition(),
			},
		)
	}

	// check message expression results in a string

	if condition.Message != nil {

		messageType := condition.Message.Accept(checker).(Type)

		if !messageType.Equal(&InvalidType{}) && !checker.IsSubType(messageType, &StringType{}) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: &StringType{},
					ActualType:   testType,
					StartPos:     condition.Message.StartPosition(),
					EndPos:       condition.Message.EndPosition(),
				},
			)
		}
	}

	return nil
}

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {

	// check value type matches enclosing function's return type

	if statement.Expression == nil {
		return nil
	}

	valueType := statement.Expression.Accept(checker).(Type)
	_, valueIsInvalid := valueType.(*InvalidType)

	returnType := checker.currentFunction().returnType

	if valueType != nil {
		if valueIsInvalid {
			// return statement has expression, but function has Void return type?
			if _, ok := returnType.(*VoidType); ok {
				checker.report(
					&InvalidReturnValueError{
						StartPos: statement.Expression.StartPosition(),
						EndPos:   statement.Expression.EndPosition(),
					},
				)
			}
		} else {
			if !checker.IsSubType(valueType, returnType) {
				checker.report(
					&TypeMismatchError{
						ExpectedType: returnType,
						ActualType:   valueType,
						StartPos:     statement.Expression.StartPosition(),
						EndPos:       statement.Expression.EndPosition(),
					},
				)
			}
		}
	}

	return nil
}

func (checker *Checker) VisitBreakStatement(statement *ast.BreakStatement) ast.Repr {

	// check statement is inside loop

	if checker.currentFunction().loops == 0 {
		checker.report(
			&ControlStatementError{
				ControlStatement: common.ControlStatementBreak,
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return nil
}

func (checker *Checker) VisitContinueStatement(statement *ast.ContinueStatement) ast.Repr {

	// check statement is inside loop

	if checker.currentFunction().loops == 0 {
		checker.report(
			&ControlStatementError{
				ControlStatement: common.ControlStatementContinue,
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return nil
}

func (checker *Checker) VisitIfStatement(statement *ast.IfStatement) ast.Repr {

	thenElement := statement.Then

	var elseElement ast.Element = ast.NotAnElement{}
	if statement.Else != nil {
		elseElement = statement.Else
	}

	switch test := statement.Test.(type) {
	case ast.Expression:
		checker.visitConditional(test, thenElement, elseElement)

	case *ast.VariableDeclaration:
		func() {
			checker.pushActivations()
			defer checker.popActivations()

			checker.visitVariableDeclaration(test, true)

			thenElement.Accept(checker)
		}()

		elseElement.Accept(checker)

	default:
		panic(&errors.UnreachableError{})
	}

	return nil
}

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {

	testExpression := statement.Test
	testType := testExpression.Accept(checker).(Type)

	if !checker.IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     testExpression.StartPosition(),
				EndPos:       testExpression.EndPosition(),
			},
		)
	}

	checker.currentFunction().loops += 1
	defer func() {
		checker.currentFunction().loops -= 1
	}()

	statement.Block.Accept(checker)

	return nil
}

func (checker *Checker) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {

	valueType := assignment.Value.Accept(checker).(Type)

	targetType := checker.visitAssignmentValueType(assignment, valueType)

	checker.AssignmentStatementTargetTypes[assignment] = targetType

	return nil
}

func (checker *Checker) visitAssignmentValueType(assignment *ast.AssignmentStatement, valueType Type) (targetType Type) {
	switch target := assignment.Target.(type) {
	case *ast.IdentifierExpression:
		return checker.visitIdentifierExpressionAssignment(assignment, target, valueType)

	case *ast.IndexExpression:
		return checker.visitIndexExpressionAssignment(assignment, target, valueType)

	case *ast.MemberExpression:
		return checker.visitMemberExpressionAssignment(assignment, target, valueType)

	default:
		panic(&unsupportedAssignmentTargetExpression{
			target: target,
		})
	}

	panic(&errors.UnreachableError{})
}

func (checker *Checker) visitIdentifierExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IdentifierExpression,
	valueType Type,
) (targetType Type) {
	identifier := target.Identifier

	// check identifier was declared before
	variable := checker.findVariable(identifier)
	if variable == nil {
		checker.report(
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindVariable,
				Name:         identifier,
				Pos:          target.StartPosition(),
			},
		)

		return &InvalidType{}
	} else {
		// check identifier is not a constant
		if variable.IsConstant {
			checker.report(
				&AssignmentToConstantError{
					Name:     identifier,
					StartPos: target.StartPosition(),
					EndPos:   target.EndPosition(),
				},
			)
		}

		// check value type is subtype of variable type
		if !valueType.Equal(&InvalidType{}) &&
			!checker.IsSubType(valueType, variable.Type) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: variable.Type,
					ActualType:   valueType,
					StartPos:     assignment.Value.StartPosition(),
					EndPos:       assignment.Value.EndPosition(),
				},
			)
		}

		return variable.Type
	}
}

func (checker *Checker) visitIndexExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.IndexExpression,
	valueType Type,
) (elementType Type) {

	elementType = checker.visitIndexingExpression(target.Expression, target.Index)

	if elementType == nil {
		return &InvalidType{}
	}

	if !elementType.Equal(&InvalidType{}) &&
		!checker.IsSubType(valueType, elementType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: elementType,
				ActualType:   valueType,
				StartPos:     assignment.Value.StartPosition(),
				EndPos:       assignment.Value.EndPosition(),
			},
		)
	}

	return elementType
}

func (checker *Checker) visitMemberExpressionAssignment(
	assignment *ast.AssignmentStatement,
	target *ast.MemberExpression,
	valueType Type,
) (memberType Type) {

	member := checker.visitMember(target)

	if member == nil {
		return
	}

	// check member is not constant

	if member.VariableKind == ast.VariableKindConstant {
		if member.IsInitialized {
			checker.report(
				&AssignmentToConstantMemberError{
					Name:     target.Identifier,
					StartPos: assignment.Value.StartPosition(),
					EndPos:   assignment.Value.EndPosition(),
				},
			)
		}
	}

	member.IsInitialized = true

	// if value type is valid, check value can be assigned to member
	if _, ok := valueType.(*InvalidType); !ok {
		if !checker.IsSubType(valueType, member.Type) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: member.Type,
					ActualType:   valueType,
					StartPos:     assignment.Value.StartPosition(),
					EndPos:       assignment.Value.EndPosition(),
				},
			)
		}
	}

	return member.Type
}

// visitIndexingExpression checks if the indexed expression is indexable,
// checks if the indexing expression can be used to index into the indexed expression,
// and returns the expected element type
//
func (checker *Checker) visitIndexingExpression(indexedExpression, indexingExpression ast.Expression) Type {

	indexedType := indexedExpression.Accept(checker).(Type)
	indexingType := indexingExpression.Accept(checker).(Type)

	// NOTE: check indexed type first for UX reasons

	// check indexed expression's type is indexable
	// by getting the expected element

	if _, ok := indexedType.(*InvalidType); ok {
		return &InvalidType{}
	}

	elementType := checker.IndexableElementType(indexedType)
	if elementType == nil {
		elementType = &InvalidType{}

		checker.report(
			&NotIndexableTypeError{
				Type:     indexedType,
				StartPos: indexedExpression.StartPosition(),
				EndPos:   indexedExpression.EndPosition(),
			},
		)
	} else {

		// check indexing expression's type can be used to index
		// into indexed expression's type

		if !checker.IsIndexingType(indexingType, indexedType) {
			checker.report(
				&NotIndexingTypeError{
					Type:     indexingType,
					StartPos: indexingExpression.StartPosition(),
					EndPos:   indexingExpression.EndPosition(),
				},
			)
		}
	}

	return elementType
}

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	variable := checker.findAndCheckVariable(expression, true)
	if variable == nil {
		return &InvalidType{}
	}

	return variable.Type
}

func (checker *Checker) findAndCheckVariable(expression *ast.IdentifierExpression, recordOrigin bool) *Variable {
	identifier := expression.Identifier
	variable := checker.findVariable(identifier)
	if variable == nil {
		checker.report(
			&NotDeclaredError{
				ExpectedKind: common.DeclarationKindValue,
				Name:         identifier,
				Pos:          expression.StartPosition(),
			},
		)
		return nil
	}

	if recordOrigin {
		checker.recordOrigin(
			expression.StartPosition(),
			expression.EndPosition(),
			variable,
		)
	}

	return variable
}

func (checker *Checker) visitBinaryOperation(expr *ast.BinaryExpression) (left, right Type) {
	left = expr.Left.Accept(checker).(Type)
	right = expr.Right.Accept(checker).(Type)
	return
}

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {

	leftType, rightType := checker.visitBinaryOperation(expression)

	_, leftIsInvalid := leftType.(*InvalidType)
	_, rightIsInvalid := rightType.(*InvalidType)
	anyInvalid := leftIsInvalid || rightIsInvalid

	operation := expression.Operation
	operationKind := binaryOperationKind(operation)

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic,
		BinaryOperationKindIntegerComparison:

		return checker.checkBinaryExpressionIntegerArithmeticOrComparison(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindEquality:

		return checker.checkBinaryExpressionEquality(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindBooleanLogic:

		return checker.checkBinaryExpressionBooleanLogic(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindNilCoalescing:

		return checker.checkBinaryExpressionNilCoalescing(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindBinary,
		operation: operation,
		startPos:  expression.StartPosition(),
		endPos:    expression.EndPosition(),
	})
}

func (checker *Checker) checkBinaryExpressionIntegerArithmeticOrComparison(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are integer subtypes

	leftIsInteger := checker.IsSubType(leftType, &IntegerType{})
	rightIsInteger := checker.IsSubType(rightType, &IntegerType{})

	if !leftIsInteger && !rightIsInteger {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}
	} else if !leftIsInteger {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &IntegerType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	} else if !rightIsInteger {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: &IntegerType{},
					ActualType:   rightType,
					StartPos:     expression.Right.StartPosition(),
					EndPos:       expression.Right.EndPosition(),
				},
			)
		}
	}
	// check both types are equal
	if !leftType.Equal(rightType) {
		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				StartPos:  expression.StartPosition(),
				EndPos:    expression.EndPosition(),
			},
		)
	}

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic:
		return leftType
	case BinaryOperationKindIntegerComparison:
		return &BoolType{}
	}

	panic(&errors.UnreachableError{})
}

func (checker *Checker) checkBinaryExpressionEquality(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) (resultType Type) {
	// check both types are equal, and boolean subtypes or integer subtypes

	resultType = &BoolType{}

	if !anyInvalid &&
		leftType != nil &&
		!(checker.isValidEqualityType(leftType) &&
			checker.compatibleEqualityTypes(leftType, rightType)) {

		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				StartPos:  expression.StartPosition(),
				EndPos:    expression.EndPosition(),
			},
		)
	}

	return
}

func (checker *Checker) isValidEqualityType(ty Type) bool {
	if checker.IsSubType(ty, &BoolType{}) {
		return true
	}

	if checker.IsSubType(ty, &IntegerType{}) {
		return true
	}

	if _, ok := ty.(*OptionalType); ok {
		return true
	}

	return false
}

func (checker *Checker) compatibleEqualityTypes(leftType, rightType Type) bool {
	unwrappedLeft := checker.unwrapOptionalType(leftType)
	unwrappedRight := checker.unwrapOptionalType(rightType)

	if unwrappedLeft.Equal(unwrappedRight) {
		return true
	}

	if _, ok := unwrappedLeft.(*NeverType); ok {
		return true
	}

	if _, ok := unwrappedRight.(*NeverType); ok {
		return true
	}

	return false
}

// unwrapOptionalType returns the type if it is not an optional type,
// or the inner-most type if it is (optional types are repeatedly unwrapped)
//
func (checker *Checker) unwrapOptionalType(ty Type) Type {
	for {
		optionalType, ok := ty.(*OptionalType)
		if !ok {
			return ty
		}
		ty = optionalType.Type
	}
}

func (checker *Checker) checkBinaryExpressionBooleanLogic(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are integer subtypes

	leftIsBool := checker.IsSubType(leftType, &BoolType{})
	rightIsBool := checker.IsSubType(rightType, &BoolType{})

	if !leftIsBool && !rightIsBool {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					StartPos:  expression.StartPosition(),
					EndPos:    expression.EndPosition(),
				},
			)
		}
	} else if !leftIsBool {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &BoolType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	} else if !rightIsBool {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: &BoolType{},
					ActualType:   rightType,
					StartPos:     expression.Right.StartPosition(),
					EndPos:       expression.Right.EndPosition(),
				},
			)
		}
	}

	return &BoolType{}
}

func (checker *Checker) checkBinaryExpressionNilCoalescing(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	leftOptional, leftIsOptional := leftType.(*OptionalType)

	if !leftIsInvalid {
		if !leftIsOptional {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &OptionalType{},
					ActualType:   leftType,
					StartPos:     expression.Left.StartPosition(),
					EndPos:       expression.Left.EndPosition(),
				},
			)
		}
	}

	if leftIsInvalid || !leftIsOptional {
		return &InvalidType{}
	}

	leftInner := leftOptional.Type

	if _, ok := leftInner.(*NeverType); ok {
		return rightType
	} else {
		canNarrow := false

		if !rightIsInvalid {
			if !checker.IsSubType(rightType, leftOptional) {
				checker.report(
					&InvalidBinaryOperandError{
						Operation:    operation,
						Side:         common.OperandSideRight,
						ExpectedType: leftOptional,
						ActualType:   rightType,
						StartPos:     expression.Right.StartPosition(),
						EndPos:       expression.Right.EndPosition(),
					},
				)
			} else {
				canNarrow = checker.IsSubType(rightType, leftInner)
			}
		}

		if !canNarrow {
			return leftOptional
		}
		return leftInner
	}
}

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {

	valueType := expression.Expression.Accept(checker).(Type)

	switch expression.Operation {
	case ast.OperationNegate:
		if !checker.IsSubType(valueType, &BoolType{}) {
			checker.report(
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &BoolType{},
					ActualType:   valueType,
					StartPos:     expression.Expression.StartPosition(),
					EndPos:       expression.Expression.EndPosition(),
				},
			)
		}
		return valueType

	case ast.OperationMinus:
		if !checker.IsSubType(valueType, &IntegerType{}) {
			checker.report(
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &IntegerType{},
					ActualType:   valueType,
					StartPos:     expression.Expression.StartPosition(),
					EndPos:       expression.Expression.EndPosition(),
				},
			)
		}
		return valueType
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindUnary,
		operation: expression.Operation,
		startPos:  expression.StartPos,
		endPos:    expression.EndPos,
	})
}

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	statement.Expression.Accept(checker)
	return nil
}

func (checker *Checker) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return &BoolType{}
}

func (checker *Checker) VisitNilExpression(expression *ast.NilExpression) ast.Repr {
	// TODO: verify
	return &OptionalType{
		Type: &NeverType{},
	}
}

func (checker *Checker) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return &IntType{}
}

func (checker *Checker) VisitStringExpression(expression *ast.StringExpression) ast.Repr {
	return &StringType{}
}

func (checker *Checker) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {

	// visit all elements, ensure they are all the same type

	var elementType Type

	for _, value := range expression.Values {
		valueType := value.Accept(checker).(Type)

		// infer element type from first element
		// TODO: find common super type?
		if elementType == nil {
			elementType = valueType
		} else if !checker.IsSubType(valueType, elementType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: elementType,
					ActualType:   valueType,
					StartPos:     value.StartPosition(),
					EndPos:       value.EndPosition(),
				},
			)
		}
	}

	// TODO: use bottom type
	if elementType == nil {
		elementType = &AnyType{}
	}

	return &VariableSizedType{
		Type: elementType,
	}
}

func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {

	member := checker.visitMember(expression)

	var memberType Type = &InvalidType{}
	if member != nil {
		memberType = member.Type
	}

	return memberType
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) *Member {

	expressionType := expression.Expression.Accept(checker).(Type)

	identifier := expression.Identifier

	var member *Member
	var ok bool
	switch ty := expressionType.(type) {
	case *StructureType:
		member, ok = ty.Members[identifier]
	case *InterfaceType:
		member, ok = ty.Members[identifier]
	}

	if !ok {
		checker.report(
			&NotDeclaredMemberError{
				Type:     expressionType,
				Name:     identifier,
				StartPos: expression.StartPos,
				EndPos:   expression.EndPos,
			},
		)
	}

	return member
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	return checker.visitIndexingExpression(expression.Expression, expression.Index)
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {

	thenType, elseType := checker.visitConditional(expression.Test, expression.Then, expression.Else)

	if thenType == nil || elseType == nil {
		panic(&errors.UnreachableError{})
	}

	// TODO: improve
	resultType := thenType

	if !checker.IsSubType(elseType, resultType) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: resultType,
				ActualType:   elseType,
				StartPos:     expression.Else.StartPosition(),
				EndPos:       expression.Else.EndPosition(),
			},
		)
	}

	return resultType
}

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {

	// check the invoked expression can be invoked

	invokedExpression := invocationExpression.InvokedExpression
	expressionType := invokedExpression.Accept(checker).(Type)

	var returnType Type = &InvalidType{}
	functionType, ok := expressionType.(*FunctionType)
	if !ok {

		if _, ok := expressionType.(*InvalidType); !ok {
			checker.report(
				&NotCallableError{
					Type:     expressionType,
					StartPos: invokedExpression.StartPosition(),
					EndPos:   invokedExpression.EndPosition(),
				},
			)
		}
	} else {
		// invoked expression has function type

		argumentTypes := checker.checkInvocationArguments(invocationExpression, functionType)

		// if the invocation refers directly to the name of the function as stated in the declaration,
		// or the invocation refers to a function of a structure (member),
		// check that the correct argument labels are supplied in the invocation

		if identifierExpression, ok := invokedExpression.(*ast.IdentifierExpression); ok {
			checker.checkIdentifierInvocationArgumentLabels(
				invocationExpression,
				identifierExpression,
			)
		} else if memberExpression, ok := invokedExpression.(*ast.MemberExpression); ok {
			checker.checkMemberInvocationArgumentLabels(
				invocationExpression,
				memberExpression,
			)
		}

		parameterTypes := functionType.ParameterTypes
		if len(argumentTypes) == len(parameterTypes) &&
			functionType.Apply != nil {

			returnType = functionType.Apply(argumentTypes)
		} else {
			returnType = functionType.ReturnType
		}

		checker.InvocationExpressionParameterTypes[invocationExpression] = parameterTypes
	}

	return returnType
}

func (checker *Checker) checkIdentifierInvocationArgumentLabels(
	invocationExpression *ast.InvocationExpression,
	identifierExpression *ast.IdentifierExpression,
) {

	variable := checker.findAndCheckVariable(identifierExpression, false)

	if variable == nil || len(variable.ArgumentLabels) == 0 {
		return
	}

	checker.checkInvocationArgumentLabels(
		invocationExpression.Arguments,
		variable.ArgumentLabels,
	)
}

func (checker *Checker) checkMemberInvocationArgumentLabels(
	invocationExpression *ast.InvocationExpression,
	memberExpression *ast.MemberExpression,
) {
	member := checker.visitMember(memberExpression)

	if member == nil || len(member.ArgumentLabels) == 0 {
		return
	}

	checker.checkInvocationArgumentLabels(
		invocationExpression.Arguments,
		member.ArgumentLabels,
	)
}

func (checker *Checker) checkInvocationArgumentLabels(
	arguments []*ast.Argument,
	argumentLabels []string,
) {
	argumentCount := len(arguments)

	for i, argumentLabel := range argumentLabels {
		if i >= argumentCount {
			break
		}

		argument := arguments[i]
		providedLabel := argument.Label
		if argumentLabel == ArgumentLabelNotRequired {
			// argument label is not required,
			// check it is not provided

			if providedLabel != "" {
				checker.report(
					&IncorrectArgumentLabelError{
						ActualArgumentLabel:   providedLabel,
						ExpectedArgumentLabel: "",
						StartPos:              *argument.LabelStartPos,
						EndPos:                *argument.LabelEndPos,
					},
				)
			}
		} else {
			// argument label is required,
			// check it is provided and correct
			if providedLabel == "" {
				checker.report(
					&MissingArgumentLabelError{
						ExpectedArgumentLabel: argumentLabel,
						StartPos:              argument.Expression.StartPosition(),
						EndPos:                argument.Expression.EndPosition(),
					},
				)
			} else if providedLabel != argumentLabel {
				checker.report(
					&IncorrectArgumentLabelError{
						ActualArgumentLabel:   providedLabel,
						ExpectedArgumentLabel: argumentLabel,
						StartPos:              *argument.LabelStartPos,
						EndPos:                *argument.LabelEndPos,
					},
				)
			}
		}
	}
}

func (checker *Checker) checkInvocationArguments(
	invocationExpression *ast.InvocationExpression,
	functionType *FunctionType,
) (
	argumentTypes []Type,
) {
	argumentCount := len(invocationExpression.Arguments)

	// check the invocation's argument count matches the function's parameter count
	parameterCount := len(functionType.ParameterTypes)
	if argumentCount != parameterCount {

		// TODO: improve
		if functionType.RequiredArgumentCount == nil ||
			argumentCount < *functionType.RequiredArgumentCount {

			checker.report(
				&ArgumentCountError{
					ParameterCount: parameterCount,
					ArgumentCount:  argumentCount,
					StartPos:       invocationExpression.StartPosition(),
					EndPos:         invocationExpression.EndPosition(),
				},
			)
		}
	}

	minCount := argumentCount
	if parameterCount < argumentCount {
		minCount = parameterCount
	}

	for i := 0; i < minCount; i++ {
		// ensure the type of the argument matches the type of the parameter

		parameterType := functionType.ParameterTypes[i]
		argument := invocationExpression.Arguments[i]

		argumentType := argument.Expression.Accept(checker).(Type)

		argumentTypes = append(argumentTypes, argumentType)

		if !checker.IsSubType(argumentType, parameterType) {
			checker.report(
				&TypeMismatchError{
					ExpectedType: parameterType,
					ActualType:   argumentType,
					StartPos:     argument.Expression.StartPosition(),
					EndPos:       argument.Expression.EndPosition(),
				},
			)
		}
	}

	return argumentTypes
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {

	// TODO: infer
	functionType := checker.functionType(expression.Parameters, expression.ReturnType)

	checker.FunctionExpressionFunctionType[expression] = functionType

	checker.checkFunction(
		expression.Parameters,
		functionType,
		expression.FunctionBlock,
	)

	// function expressions are not allowed in conditions

	if checker.inCondition {
		checker.report(
			&FunctionExpressionInConditionError{
				StartPos: expression.StartPosition(),
				EndPos:   expression.EndPosition(),
			},
		)
	}

	return functionType
}

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case *ast.NominalType:
		result := checker.FindType(t.Identifier)
		if result == nil {
			checker.report(
				&NotDeclaredError{
					ExpectedKind: common.DeclarationKindType,
					Name:         t.Identifier,
					Pos:          t.Pos,
				},
			)
			return &InvalidType{}
		}
		return result

	case *ast.VariableSizedType:
		elementType := checker.ConvertType(t.Type)
		return &VariableSizedType{
			Type: elementType,
		}

	case *ast.ConstantSizedType:
		elementType := checker.ConvertType(t.Type)
		return &ConstantSizedType{
			Type: elementType,
			Size: t.Size,
		}

	case *ast.FunctionType:
		var parameterTypes []Type
		for _, parameterType := range t.ParameterTypes {
			parameterType := checker.ConvertType(parameterType)
			parameterTypes = append(parameterTypes, parameterType)
		}

		returnType := checker.ConvertType(t.ReturnType)

		return &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     returnType,
		}

	case *ast.OptionalType:
		result := checker.ConvertType(t.Type)
		return &OptionalType{result}
	}

	panic(&astTypeConversionError{invalidASTType: t})
}

func (checker *Checker) declareFunction(
	identifier string,
	identifierPosition ast.Position,
	functionType *FunctionType,
	argumentLabels []string,
	recordOrigin bool,
) {

	// check if variable with this identifier is already declared in the current scope
	existingVariable := checker.findVariable(identifier)
	depth := checker.valueActivations.Depth()
	if existingVariable != nil && existingVariable.Depth == depth {
		checker.report(
			&RedeclarationError{
				Kind:        common.DeclarationKindFunction,
				Name:        identifier,
				Pos:         identifierPosition,
				PreviousPos: existingVariable.Pos,
			},
		)
	}

	// variable with this identifier is not declared in current scope, declare it
	variable := &Variable{
		Kind:           common.DeclarationKindFunction,
		IsConstant:     true,
		Depth:          depth,
		Type:           functionType,
		ArgumentLabels: argumentLabels,
		Pos:            &identifierPosition,
	}
	checker.setVariable(identifier, variable)
	if recordOrigin {
		checker.recordVariableOrigin(identifier, variable)
	}
}

func (checker *Checker) enterFunction(functionType *FunctionType) {
	checker.functionContexts = append(checker.functionContexts,
		&functionContext{
			returnType: functionType.ReturnType,
		})
}

func (checker *Checker) leaveFunction() {
	lastIndex := len(checker.functionContexts) - 1
	checker.functionContexts = checker.functionContexts[:lastIndex]
}

func (checker *Checker) currentFunction() *functionContext {
	lastIndex := len(checker.functionContexts) - 1
	if lastIndex < 0 {
		return nil
	}
	return checker.functionContexts[lastIndex]
}

func (checker *Checker) functionType(parameters []*ast.Parameter, returnType ast.Type) *FunctionType {

	parameterTypes := checker.parameterTypes(parameters)
	convertedReturnType := checker.ConvertType(returnType)

	return &FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     convertedReturnType,
	}
}

func (checker *Checker) parameterTypes(parameters []*ast.Parameter) []Type {

	parameterTypes := make([]Type, len(parameters))

	for i, parameter := range parameters {
		parameterType := checker.ConvertType(parameter.Type)
		parameterTypes[i] = parameterType
	}

	return parameterTypes
}

// visitConditional checks a conditional. the test expression must be a boolean.
// the then and else elements may be expressions, in which case the types are returned.
func (checker *Checker) visitConditional(
	test ast.Expression,
	thenElement ast.Element,
	elseElement ast.Element,
) (
	thenType, elseType Type,
) {
	testType := test.Accept(checker).(Type)

	if !checker.IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     test.StartPosition(),
				EndPos:       test.EndPosition(),
			},
		)
	}

	thenResult := thenElement.Accept(checker)
	if thenResult != nil {
		thenType = thenResult.(Type)
	}

	elseResult := elseElement.Accept(checker)
	if elseResult != nil {
		elseType = elseResult.(Type)
	}

	return
}

func (checker *Checker) VisitStructureDeclaration(structure *ast.StructureDeclaration) ast.Repr {

	structureType := checker.StructureDeclarationTypes[structure]

	checker.checkMemberNames(structure.Fields, structure.Functions)

	checker.checkInitializer(
		structure.Initializer,
		structure.Fields,
		structureType,
		structure.Identifier,
		structureType.ConstructorParameterTypes,
		initializerKindStructure,
	)

	if structureType != nil {
		checker.checkFieldsInitialized(structure, structureType)
	}

	checker.checkStructureFunctions(structure.Functions, structureType)

	return nil
}

func (checker *Checker) declareStructureDeclaration(structure *ast.StructureDeclaration) {

	// NOTE: fields and functions might already refer to structure itself.
	// insert a dummy type for now, so lookup succeeds during conversion,
	// then fix up the type reference

	structureType := &StructureType{}

	checker.declareType(
		structure.Identifier,
		structure.IdentifierPos,
		structureType,
		common.DeclarationKindStructure,
	)

	conformances := checker.conformances(structure.Conformances)

	members := checker.members(
		structure.Fields,
		structure.Functions,
		common.DeclarationKindStructure,
	)

	*structureType = StructureType{
		Identifier:   structure.Identifier,
		Members:      members,
		Conformances: conformances,
	}

	checker.StructureDeclarationTypes[structure] = structureType

	// declare constructor

	initializer := structure.Initializer
	var parameterTypes []Type
	if initializer != nil {
		parameterTypes = checker.parameterTypes(initializer.Parameters)
	}

	checker.declareStructureConstructor(structure, structureType, parameterTypes)

	structureType.ConstructorParameterTypes = parameterTypes
}

func (checker *Checker) conformances(conformances []*ast.NominalType) []*InterfaceType {
	var interfaceTypes []*InterfaceType
	for _, conformance := range conformances {
		convertedType := checker.ConvertType(conformance)

		if interfaceType, ok := convertedType.(*InterfaceType); ok {
			interfaceTypes = append(interfaceTypes, interfaceType)

		} else if _, ok := convertedType.(*InvalidType); !ok {
			checker.report(
				&InvalidConformanceError{
					Type: convertedType,
					Pos:  conformance.Pos,
				},
			)
		}
	}
	return interfaceTypes
}

// TODO: very simple field initialization check for now.
//  perform proper definite assignment analysis
//
func (checker *Checker) checkFieldsInitialized(
	structure *ast.StructureDeclaration,
	structureType *StructureType,
) {

	for _, field := range structure.Fields {
		name := field.Identifier
		member := structureType.Members[name]

		if !member.IsInitialized {
			checker.report(
				&FieldUninitializedError{
					Name:          name,
					Pos:           field.IdentifierPos,
					StructureType: structureType,
				},
			)
		}
	}
}

func (checker *Checker) declareStructureConstructor(
	structure *ast.StructureDeclaration,
	structureType *StructureType,
	parameterTypes []Type,
) {
	functionType := &FunctionType{
		ReturnType: structureType,
	}

	var argumentLabels []string

	initializer := structure.Initializer
	if initializer != nil {
		argumentLabels = checker.argumentLabels(initializer.Parameters)

		functionType = &FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     structureType,
		}
	}

	checker.InitializerFunctionTypes[initializer] = functionType

	checker.declareFunction(
		structure.Identifier,
		structure.IdentifierPos,
		functionType,
		argumentLabels,
		false,
	)
}

func (checker *Checker) members(
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
	declarationKind common.DeclarationKind,
) map[string]*Member {

	fieldCount := len(fields)
	functionCount := len(functions)

	members := make(map[string]*Member, fieldCount+functionCount)

	// declare a member for each field
	for _, field := range fields {
		fieldType := checker.ConvertType(field.Type)

		members[field.Identifier] = &Member{
			Type:          fieldType,
			VariableKind:  field.VariableKind,
			IsInitialized: false,
		}

		if field.VariableKind == ast.VariableKindNotSpecified &&
			declarationKind != common.DeclarationKindInterface {

			checker.report(
				&InvalidVariableKindError{
					Kind:     field.VariableKind,
					StartPos: field.IdentifierPos,
					EndPos:   field.IdentifierPos,
				},
			)
		}
	}

	// declare a member for each function
	for _, function := range functions {
		functionType := checker.functionType(function.Parameters, function.ReturnType)

		argumentLabels := checker.argumentLabels(function.Parameters)

		members[function.Identifier] = &Member{
			Type:           functionType,
			VariableKind:   ast.VariableKindConstant,
			IsInitialized:  true,
			ArgumentLabels: argumentLabels,
		}
	}

	return members
}

func (checker *Checker) checkInitializer(
	initializer *ast.InitializerDeclaration,
	fields []*ast.FieldDeclaration,
	ty Type,
	typeIdentifier string,
	constructorParameterTypes []Type,
	kind initializerKind,
) {
	if initializer == nil {
		// no initializer, inside a structure, but fields?
		if kind != initializerKindInterface && len(fields) > 0 {
			firstField := fields[0]

			// structure has fields, but no initializer
			checker.report(
				&MissingInitializerError{
					TypeIdentifier: typeIdentifier,
					FirstFieldName: firstField.Identifier,
					FirstFieldPos:  firstField.IdentifierPos,
				},
			)
		}
		return
	}

	// NOTE: new activation, so `self`
	// is only visible inside initializer

	checker.valueActivations.PushCurrent()
	defer checker.valueActivations.Pop()

	checker.declareSelfValue(ty)

	// check the initializer is named properly
	identifier := initializer.Identifier
	if identifier != InitializerIdentifier {
		checker.report(
			&InvalidInitializerNameError{
				Name: identifier,
				Pos:  initializer.StartPos,
			},
		)
	}

	functionType := &FunctionType{
		ParameterTypes: constructorParameterTypes,
		ReturnType:     &VoidType{},
	}

	checker.checkFunction(
		initializer.Parameters,
		functionType,
		initializer.FunctionBlock,
	)

	if kind == initializerKindInterface &&
		initializer.FunctionBlock != nil {

		checker.checkInterfaceFunctionBlock(
			initializer.FunctionBlock,
			common.DeclarationKindInitializer,
		)
	}
}

func (checker *Checker) checkStructureFunctions(
	functions []*ast.FunctionDeclaration,
	selfType *StructureType,
) {
	for _, function := range functions {
		func() {
			// NOTE: new activation, as function declarations
			// shouldn'T be visible in other function declarations,
			// and `self` is is only visible inside function
			checker.valueActivations.PushCurrent()
			defer checker.valueActivations.Pop()

			checker.declareSelfValue(selfType)

			function.Accept(checker)
		}()
	}
}

func (checker *Checker) declareSelfValue(selfType Type) {

	// NOTE: declare `self` one depth lower ("inside" function),
	// so it can'T be re-declared by the function's parameters

	depth := checker.valueActivations.Depth() + 1

	self := &Variable{
		Kind:       common.DeclarationKindConstant,
		Type:       selfType,
		IsConstant: true,
		Depth:      depth,
		Pos:        nil,
	}
	checker.setVariable(SelfIdentifier, self)
	checker.recordVariableOrigin(SelfIdentifier, self)
}

// checkMemberNames checks the fields and functions are unique and aren'T named `init`
//
func (checker *Checker) checkMemberNames(
	fields []*ast.FieldDeclaration,
	functions []*ast.FunctionDeclaration,
) {

	positions := map[string]ast.Position{}

	for _, field := range fields {
		checker.checkMemberName(
			field.Identifier,
			field.IdentifierPos,
			common.DeclarationKindField,
			positions,
		)
	}

	for _, function := range functions {
		checker.checkMemberName(
			function.Identifier,
			function.IdentifierPos,
			common.DeclarationKindFunction,
			positions,
		)
	}
}

func (checker *Checker) checkMemberName(
	name string,
	pos ast.Position,
	kind common.DeclarationKind,
	positions map[string]ast.Position,
) {

	if name == InitializerIdentifier {
		checker.report(
			&InvalidNameError{
				Name: name,
				Pos:  pos,
			},
		)
	}

	if previousPos, ok := positions[name]; ok {
		checker.report(
			&RedeclarationError{
				Name:        name,
				Pos:         pos,
				Kind:        kind,
				PreviousPos: &previousPos,
			},
		)
	} else {
		positions[name] = pos
	}
}

func (checker *Checker) declareType(
	identifier string,
	identifierPos ast.Position,
	newType Type,
	kind common.DeclarationKind,
) {
	existingType := checker.FindType(identifier)
	if existingType != nil {
		checker.report(
			&RedeclarationError{
				Kind: common.DeclarationKindType,
				Name: identifier,
				Pos:  identifierPos,
				// TODO: previous pos
			},
		)
	}

	// type with this identifier is not declared in current scope, declare it
	checker.setType(identifier, newType)
	checker.recordVariableOrigin(
		identifier,
		&Variable{
			Kind:       kind,
			IsConstant: true,
			Type:       newType,
			Pos:        &identifierPos,
		},
	)
}

func (checker *Checker) VisitFieldDeclaration(field *ast.FieldDeclaration) ast.Repr {

	// NOTE: field type is already checked when determining structure function in `structureType`

	panic(&errors.UnreachableError{})
}

func (checker *Checker) VisitInitializerDeclaration(initializer *ast.InitializerDeclaration) ast.Repr {

	// NOTE: already checked in `checkInitializer`

	panic(&errors.UnreachableError{})
}

func (checker *Checker) VisitInterfaceDeclaration(declaration *ast.InterfaceDeclaration) ast.Repr {

	interfaceType := checker.InterfaceDeclarationTypes[declaration]

	checker.checkMemberNames(declaration.Fields, declaration.Functions)

	checker.checkInitializer(
		declaration.Initializer,
		declaration.Fields,
		interfaceType,
		declaration.Identifier,
		interfaceType.ConstructorParameterTypes,
		initializerKindInterface,
	)

	checker.checkInterfaceFunctions(declaration.Functions, interfaceType)

	return nil
}

func (checker *Checker) checkInterfaceFunctions(functions []*ast.FunctionDeclaration, interfaceType Type) {
	for _, function := range functions {
		func() {
			// NOTE: new activation, as function declarations
			// shouldn'T be visible in other function declarations,
			// and `self` is is only visible inside function
			checker.valueActivations.PushCurrent()
			defer checker.valueActivations.Pop()

			// NOTE: required for
			checker.declareSelfValue(interfaceType)

			function.Accept(checker)

			if function.FunctionBlock != nil {
				checker.checkInterfaceFunctionBlock(
					function.FunctionBlock,
					common.DeclarationKindFunction,
				)
			}
		}()
	}
}

func (checker *Checker) declareInterfaceDeclaration(declaration *ast.InterfaceDeclaration) {

	// NOTE: fields and functions might already refer to structure itself.
	// insert a dummy type for now, so lookup succeeds during conversion,
	// then fix up the type reference

	interfaceType := &InterfaceType{}

	checker.declareType(
		declaration.Identifier,
		declaration.IdentifierPos,
		interfaceType,
		common.DeclarationKindInterface,
	)

	members := checker.members(
		declaration.Fields,
		declaration.Functions,
		common.DeclarationKindInterface,
	)

	*interfaceType = InterfaceType{
		Identifier: declaration.Identifier,
		Members:    members,
	}

	checker.InterfaceDeclarationTypes[declaration] = interfaceType

	// declare constructor

	initializer := declaration.Initializer
	var parameterTypes []Type
	if initializer != nil {
		parameterTypes = checker.parameterTypes(initializer.Parameters)
	}

	interfaceType.ConstructorParameterTypes = parameterTypes
}

func (checker *Checker) checkInterfaceFunctionBlock(block *ast.FunctionBlock, kind common.DeclarationKind) {

	if len(block.Statements) > 0 {
		checker.report(
			&InvalidImplementationError{
				Pos:             block.Statements[0].StartPosition(),
				ContainerKind:   common.DeclarationKindInterface,
				ImplementedKind: kind,
			},
		)
	} else if len(block.PreConditions) == 0 &&
		len(block.PostConditions) == 0 {

		checker.report(
			&InvalidImplementationError{
				Pos:             block.StartPos,
				ContainerKind:   common.DeclarationKindInterface,
				ImplementedKind: kind,
			},
		)
	}
}

func (checker *Checker) recordOrigin(startPos, endPos ast.Position, variable *Variable) {
	checker.Origins.Put(startPos, endPos, variable)
}

func (checker *Checker) recordVariableOrigin(name string, variable *Variable) {
	if variable.Pos == nil {
		return
	}
	startPos := *variable.Pos
	endPos := variable.Pos.Shifted(len(name) - 1)
	checker.recordOrigin(startPos, endPos, variable)
}
